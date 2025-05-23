package compose

import (
	"context"
	"log"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
)

func Parse(composeFile string) *types.Project {
	options, err := cli.NewProjectOptions(
		[]string{composeFile},
		cli.WithOsEnv,
		cli.WithDotEnv,
		cli.WithName("ieltsallin"), // TODO: make this configurable
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	project, err := options.LoadProject(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return project
}
