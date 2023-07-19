package application

import (
	"keess/kube_syncer"

	"github.com/urfave/cli/v2"
)

// Creates a new instance of the program to run.
func New() *cli.App {
	app := cli.NewApp()
	app.Name = "Keess"
	app.Version = "v.0.0.1"
	app.Usage = "Keep stuffs synchronized."
	app.Description = "Keep secrets and configmaps synchronized."
	app.Suggest = true
	app.Authors = []*cli.Author{
		{
			Name:  "Marcus Vinicius Leandro",
			Email: "mvleandro@gmail.com",
		},
	}
	app.Copyright = "Powerhrg"

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "config",
			Usage: "Path to the kubeconfig file",
		},
		&cli.StringFlag{
			Name:     "sourceContext",
			Usage:    "The context to be watched.",
			Required: true,
		},
		&cli.StringSliceFlag{
			Name:     "destinationContexts",
			Usage:    "A list with the contexts where the events will by synched to",
			Required: false,
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:   "run",
			Usage:  "Keep secrets and configmaps syncronized across clusters and namespaces",
			Flags:  flags,
			Action: run,
		},
	}

	return app
}

// Run the program.
func run(c *cli.Context) error {
	kubeConfigPath := c.String("config")
	sourceContext := c.String("sourceContext")
	destinationContexts := c.StringSlice("destinationContexts")

	var syncer kube_syncer.Syncer
	err := syncer.Start(kubeConfigPath, sourceContext, destinationContexts)

	if err == nil {
		return syncer.Run()
	}

	return nil
}
