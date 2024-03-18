package main

import (
	"base64"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"

	"github.com/hashicorp/go-version"
	cli "github.com/urfave/cli/v2"
	_ "github.com/mattn/go-sqlite3"
	"dkforest/pkg/actions"
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"embed"
)

// These variables are overwritten during the build process using ldflags
// "version" is base64 encoded to make it harder for a hacker to change
// the value by simply ctrl+f & replace the compiled binary file.
var (
	version               = "MTAwMC4wLjAK" // Base6
