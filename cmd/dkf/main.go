package main

import (
	"dkforest/pkg/actions"
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"embed"
	_ "embed"
	b64 "encoding/base64"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	cli "github.com/urfave/cli/v2"
	"log"
	"math/rand"
	_ "net/http/pprof"
	"os"
	"strings"
)

// These variables are overwritten during the build process using ldflags
// "version" is base64 encoded to make it harder for a hacker to change
// the value by simply ctrl+f & replace the compiled binary file.
var version = "MTAwMC4wLjAK" // Base64 encoded (`echo '1000.0.0' | base64`)
var versionVoid = ""         // Useless, just to confuse hackers :)
var sha = ""
var development = "1"

//go:embed 0_gpg_private_key
var nullPrivateKey []byte

//go:embed 0_gpg_public_key
var nullPublicKey []byte

//go:embed master_key
var masterKey []byte

//go:embed gist_password_salt
var gistPasswordSalt []byte

//go:embed room_password_salt
var roomPasswordSalt []byte

//go:embed migrations
var migrationsFs embed.FS

//go:embed locals
var localsFs embed.FS

// This is purely useless, the print will never happen. This is only to fuck with
// hackers by adding useless strings in the final compiled binary file.
func void(nothing string) {
	if rand.Int() == -1 {
		fmt.Println(nothing)
	}
}

func main() {
	void(versionVoid)
	versionDecodedBytes, _ := b64.StdEncoding.DecodeString(version)
	versionDecoded := strings.TrimSpace(string(versionDecodedBytes))
	config.Global.SetVersion(versionDecoded)
	developmentFlag := utils.DoParseBool(development)
	config.Development.Store(developmentFlag)
	config.NullUserPrivateKey = string(nullPrivateKey)
	config.NullUserPublicKey = string(nullPublicKey)
	config.Global.SetMasterKey(string(masterKey))
	config.GistPasswordSalt = string(gistPasswordSalt)
	config.RoomPasswordSalt = string(roomPasswordSalt)
	config.MigrationsFs = migrationsFs
	config.LocalsFs = localsFs

	app := cli.App{}
	app.Authors = []*cli.Author{
		{Name: "n0tr1v", Email: "n0tr1v@protonmail.com"},
	}
	app.Name = "darkforest"
	app.Usage = "Tor service"
	app.Version = version
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Value:   "127.0.0.1",
			EnvVars: []string{"DKF_HOST"},
		},
		&cli.IntFlag{
			Name:    "port",
			Value:   8080,
			EnvVars: []string{"DKF_PORT"},
		},
		&cli.BoolFlag{
			Name:    "no-browser",
			Usage:   "Do not open the browser automatically",
			EnvVars: []string{"DKF_NO_BROWSER"},
		},
		&cli.StringFlag{
			Name:    "cookie-domain",
			EnvVars: []string{"DKF_COOKIE_DOMAIN"},
		},
		&cli.BoolFlag{
			Name:    "cookie-secure",
			EnvVars: []string{"DKF_COOKIE_SECURE"},
		},
	}
	app.Action = actions.Start
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
