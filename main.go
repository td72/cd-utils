package main
import (
	"os"
	"github.com/urfave/cli"
)


var (
	App = cli.NewApp()
)



func initialize(_ *cli.Context) error {
	return nil
}


func main() {
	App.Name = "cd-utils"
	App.Usage = "cd-utils"
	App.Version = "0.0.1"
	App.Before = initialize
	App.Setup()
	App.Run(os.Args)
}
