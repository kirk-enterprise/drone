package main

import (
	"encoding/json"
	sflag "flag"
	"fmt"
	"github.com/drone/drone/drone/agent"
	"github.com/drone/drone/version"
	"io/ioutil"
	"os"

	"github.com/codegangsta/cli"
	"github.com/ianschenck/envflag"
	_ "github.com/joho/godotenv/autoload"
)

func addConfEnv(path string) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	env := make(map[string]string)
	err = json.Unmarshal(file, &env)
	if err != nil {
		fmt.Println(err)
		return
	}
	for name, value := range env {
		fmt.Println(name, value)
		os.Setenv(name, value)
	}
	return
}

func main() {
	envflag.Parse()
	// 注入来自 -f 配置文件的环境变量
	confFile := sflag.String("f", "./drone.conf", "config file name")
	sflag.Parse()
	addConfEnv(*confFile)

	app := cli.NewApp()
	app.Name = "drone"
	app.Version = version.Version
	app.Usage = "command line utility"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "t, token",
			Usage:  "server auth token",
			EnvVar: "DRONE_TOKEN",
		},
		cli.StringFlag{
			Name:   "s, server",
			Usage:  "server location",
			EnvVar: "DRONE_SERVER",
		},
		cli.StringFlag{
			Name:   "f, conf",
			Usage:  "conf location",
			EnvVar: "DRONE_CONF",
		},
	}
	app.Commands = []cli.Command{
		agent.AgentCmd,
		agentsCmd,
		buildCmd,
		deployCmd,
		execCmd,
		infoCmd,
		secretCmd,
		serverCmd,
		signCmd,
		repoCmd,
		userCmd,
		orgCmd,
		globalCmd,
	}

	app.Run(os.Args)
}
