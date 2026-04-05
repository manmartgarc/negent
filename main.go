package main

import "github.com/manmart/negent/cmd"

var version = "dev"

func main() {
	cmd.RootCommand().Version = version
	cmd.Execute()
}
