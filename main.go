package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"os"
	"slices"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"
	"github.com/ttab/flerr"
)

func main() {
	if err := run(); err != nil {
		println(err.Error())
		os.Exit(1)
	}
}

func run() (outErr error) {
	ctx := context.Background()

	actor := os.Getenv("GITHUB_ACTOR")
	if actor == "" {
		return fmt.Errorf("missing GITHUB_ACTOR variable")
	}

	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		return fmt.Errorf("missing GITHUB_REPOSITORY variable")
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		return fmt.Errorf("missing GITHUB_TOKEN variable")
	}

	docker, err := client.New(client.FromEnv)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	dockerAuth, err := registryAuth(actor, ghToken)
	if err != nil {
		return fmt.Errorf("create docker auth token: %w", err)
	}

	ghcr := NewGHCRClient(http.DefaultClient, ghToken)

	conf, err := LoadImageConfig("repositories.yaml")
	if err != nil {
		return fmt.Errorf("load repository info: %w", err)
	}

	var clean flerr.Cleaner

	defer clean.FlushTo(&outErr)

	for image, conf := range conf.Images {
		// source := "minio/minio"
		// tag := "RELEASE.2025-09-07T16-13-09Z"

		println("checking", image)

		tags, err := ghcr.ListTags(ctx, repo, image)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("list tags: %w", err)
		}

		pull := slices.Clone(conf.Tags)

		pull = slices.DeleteFunc(pull, func(t string) bool {
			return slices.Contains(tags, t)
		})

		for _, tag := range pull {
			pullRef := conf.Source + ":" + tag

			println("pulling", pullRef)

			pullRes, err := docker.ImagePull(ctx, pullRef, client.ImagePullOptions{})
			if err != nil {
				return fmt.Errorf("pull image %q: %w", pullRef, err)
			}

			clean.Addf(pullRes.Close, "close pull response")

			err = drainJSONMessages(pullRes.JSONMessages(ctx))
			if err != nil {
				return fmt.Errorf("pull operation: %w", err)
			}

			err = clean.Flush()
			if err != nil {
				return err
			}

			dstRef := GHCRImage(repo, image, tag)

			_, err = docker.ImageTag(ctx, client.ImageTagOptions{
				Source: pullRef,
				Target: dstRef,
			})
			if err != nil {
				return fmt.Errorf("re-tag image: %w", err)
			}

			println("pushing", dstRef)

			pushRes, err := docker.ImagePush(ctx, dstRef, client.ImagePushOptions{
				RegistryAuth: dockerAuth,
			})
			if err != nil {
				return fmt.Errorf("push image %q: %w", pullRef, err)
			}

			clean.Addf(pushRes.Close, "close push response")

			err = drainJSONMessages(pushRes.JSONMessages(ctx))
			if err != nil {
				return fmt.Errorf("push operation: %w", err)
			}

			err = clean.Flush()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func drainJSONMessages(
	msg iter.Seq2[jsonstream.Message, error],
) error {
	for msg, err := range msg {
		if err != nil {
			return fmt.Errorf("read status message: %w", err)
		}

		if msg.Error != nil {
			return fmt.Errorf("operation failed: %s", msg.Error.Message)
		}

		if msg.ID != "" {
			print(msg.ID + ": ")
		}

		println(msg.Status)
	}

	return nil
}

func registryAuth(user, password string) (string, error) {
	data, err := json.Marshal(map[string]string{
		"username": user,
		"password": password,
	})
	if err != nil {
		return "", fmt.Errorf("marshal registry auth: %w", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}
