package docker

import (
	"context"

	"changeme/internal/model"
)

type inUseIndex struct {
	imageIDs map[string]struct{}

	volumes map[string]struct{}
}

func emptyInUseIndex() inUseIndex {
	return inUseIndex{
		imageIDs: make(map[string]struct{}),
		volumes:  make(map[string]struct{}),
	}
}

func (s *Service) containerInUseIndex(ctx context.Context) (inUseIndex, error) {
	output, err := s.execute(ctx, dockerContainerIDsCommand())
	if err != nil {
		return emptyInUseIndex(), inUseFailure(ctx)
	}
	containerIDs := parseContainerIDs(output)
	if len(containerIDs) == 0 {

		return emptyInUseIndex(), nil
	}
	index := emptyInUseIndex()
	for start := 0; start < len(containerIDs); start += inUseInspectBatchSize {
		end := start + inUseInspectBatchSize
		if end > len(containerIDs) {
			end = len(containerIDs)
		}
		args, err := dockerInUseInspectCommand(containerIDs[start:end])
		if err != nil {

			return emptyInUseIndex(), inUseFailure(ctx)
		}
		batchOutput, err := s.execute(ctx, args)
		if err != nil {
			return emptyInUseIndex(), inUseFailure(ctx)
		}
		batchImages, batchVolumes := parseInUseIndex(batchOutput)
		for id := range batchImages {
			index.imageIDs[id] = struct{}{}
		}
		for name := range batchVolumes {
			index.volumes[name] = struct{}{}
		}
	}
	return index, nil
}

func emptyNetworkInUseIndex() map[string]struct{} {
	return make(map[string]struct{}, 8)
}

func (s *Service) containerNetworkInUseIndex(ctx context.Context) (map[string]struct{}, error) {
	output, err := s.execute(ctx, dockerContainerIDsCommand())
	if err != nil {
		return emptyNetworkInUseIndex(), inUseFailure(ctx)
	}
	containerIDs := parseContainerIDs(output)
	if len(containerIDs) == 0 {
		return emptyNetworkInUseIndex(), nil
	}
	names := emptyNetworkInUseIndex()
	for start := 0; start < len(containerIDs); start += inUseInspectBatchSize {
		end := start + inUseInspectBatchSize
		if end > len(containerIDs) {
			end = len(containerIDs)
		}
		args, err := dockerNetworkInUseInspectCommand(containerIDs[start:end])
		if err != nil {

			return emptyNetworkInUseIndex(), inUseFailure(ctx)
		}
		batchOutput, err := s.execute(ctx, args)
		if err != nil {
			return emptyNetworkInUseIndex(), inUseFailure(ctx)
		}
		for name := range parseNetworkInUse(batchOutput) {
			names[name] = struct{}{}
		}
	}
	return names, nil
}

func inUseFailure(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func markImageInUse(images []model.DockerImage, index inUseIndex) {
	for position := range images {
		id := normalizeImageID(images[position].ID)
		_, inUse := index.imageIDs[id]
		images[position].InUse = id != "" && inUse
	}
}

func markVolumeInUse(volumes []model.DockerVolume, index inUseIndex) {
	for position := range volumes {
		_, inUse := index.volumes[volumes[position].Name]
		volumes[position].InUse = inUse
	}
}
