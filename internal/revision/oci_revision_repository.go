package revision

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"oma/models"
	"strings"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/zerolog/log"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

type OCIRevisionRepositoryConfig struct {
	BaseURL  string
	Username string
	Password string
}

func (c *OCIRevisionRepositoryConfig) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("REVISION_CONFIG_OCI_BASE_URL is required")
	}

	return nil
}

type OCIRevisionRepository struct {
	config   *OCIRevisionRepositoryConfig
	registry *remote.Registry
}

func NewOCIRevisionRepository(config *OCIRevisionRepositoryConfig) (*OCIRevisionRepository, error) {
	registry, err := remote.NewRegistry(config.BaseURL)
	if err != nil {
		return nil, err
	}

	registry.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(config.BaseURL, auth.Credential{
			Username: config.Username,
			Password: config.Password,
		}),
	}

	err = registry.Ping(context.Background())
	if err != nil {
		return nil, err
	}

	return &OCIRevisionRepository{
		config:   config,
		registry: registry,
	}, nil
}

func (r *OCIRevisionRepository) ListRevisions() ([]models.Revision, error) {
	repositories := []string{}
	r.registry.Repositories(context.Background(), "", func(repos []string) error {
		repositories = append(repositories, repos...)
		return nil
	})

	store := memory.New()
	revisions := []models.Revision{}
	for _, repository := range repositories {
		packageId := fmt.Sprintf("%s/%s", r.config.BaseURL, repository)
		repo, err := remote.NewRepository(packageId)
		if err != nil {
			return nil, err
		}

		repo.Tags(context.Background(), "", func(tags []string) error {
			for _, tag := range tags {
				layersManifest, err := Manifest(repo, store, tag)
				if err != nil {
					log.Error().Err(err).Msg("failed to get manifest")
					continue
				}

				var bundleDigest string
				for _, layer := range layersManifest.Layers {
					if layer.MediaType == "application/vnd.oci.image.layer.v1.tar+gzip" {
						bundleDigest = layer.Digest.Hex()
					}
				}

				// If there is no bundle digest, it is not a OPA bundle.
				if bundleDigest == "" {
					continue
				}

				createdAt, err := time.Parse(time.RFC3339, layersManifest.Annotations["org.opencontainers.image.created"])
				if err != nil {
					log.Error().Err(err).Msg("failed to parse created at")
					continue
				}

				revisions = append(revisions, models.Revision{
					Name:        repository,
					Version:     tag,
					PackageType: "oci",
					PackageId:   fmt.Sprintf("%s:%s", packageId, tag),
					CreatedAt:   createdAt,
				})

			}
			return nil
		})
	}

	return revisions, nil
}

func (r *OCIRevisionRepository) ListRevisionFiles(packageId string) ([]string, error) {
	return []string{}, nil
}

func (r *OCIRevisionRepository) DownloadRevisionById(revisionId string) (*models.Bundle, error) {
	return nil, nil
}

func (r *OCIRevisionRepository) DownloadRevision(revision *models.Revision) (*models.Bundle, error) {
	// store := memory.New()

	// // 1. Connect to a remote repository
	// ctx := context.Background()
	// fullURL, err := url.JoinPath(r.config.BaseURL, revision.PackageId)
	// if err != nil {
	// 	return nil, err
	// }

	// repo, err := remote.NewRepository(fullURL)
	// if err != nil {
	// 	return nil, err
	// }

	// // 2. Copy from the remote repository to the OCI layout store
	// tag := "local"
	// manifestDescriptor, err := oras.Copy(ctx, repo, tag, store, tag, oras.DefaultCopyOptions)
	// if err != nil {
	// 	return nil, err
	// }
	// fmt.Println("manifest descriptor:", manifestDescriptor)

	// // 3. Get all layers and look for bundle.tar.gz
	// layers, err := store.Fetch(ctx, manifestDescriptor)
	// if err != nil {
	// 	return nil, err
	// }

	// layerBytes, err := io.ReadAll(layers)
	// if err != nil {
	// 	return nil, err
	// }

	// // Read the layers into ocispec.Manifest
	// var m ocispec.Manifest
	// if err := json.Unmarshal(layerBytes, &m); err != nil {
	// 	log.Fatal().Err(err).Msg("unmarshalling layers into manifest")
	// }

	// log.Info().Msgf("Loaded bundle: %s", bundle)

	return nil, nil
}

func (r *OCIRevisionRepository) DownloadRevisionForPackage(repository string, packageAndTag string) (*models.Bundle, error) {
	fullName := fmt.Sprintf("%s/%s", repository, packageAndTag)
	tag := strings.Split(fullName, ":")[1]
	repository = strings.Split(fullName, ":")[0]

	repo, err := remote.NewRepository(repository)
	if err != nil {
		return nil, err
	}

	repo.Client = r.registry.Client

	store := memory.New()
	manifest, err := Manifest(repo, store, tag)
	if err != nil {
		return nil, err
	}

	var bundleLayer *ocispec.Descriptor
	for _, layer := range manifest.Layers {
		if layer.MediaType == "application/vnd.oci.image.layer.v1.tar+gzip" {
			bundleLayer = &layer
		}
	}

	if bundleLayer == nil {
		return nil, fmt.Errorf("bundle layer not found")
	}

	fileReader, err := store.Fetch(context.TODO(), *bundleLayer)
	if err != nil {
		return nil, err
	}

	bundle, err := UnGzTar(fileReader)
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

func Manifest(repo *remote.Repository, store oras.Target, tag string) (*ocispec.Manifest, error) {
	manifestDescriptor, err := oras.Copy(context.Background(), repo, tag, store, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, err
	}

	layerReader, err := store.Fetch(context.Background(), manifestDescriptor)
	if err != nil {
		return nil, err
	}

	layersManifestJSON, err := io.ReadAll(layerReader)
	if err != nil {
		return nil, err
	}

	// Read the layers into ocispec.Manifest
	var layersManifest ocispec.Manifest
	if err := json.Unmarshal(layersManifestJSON, &layersManifest); err != nil {
		return nil, err
	}

	return &layersManifest, nil
}
