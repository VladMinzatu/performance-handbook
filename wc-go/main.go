package main

import (
	"github.com/VladMinzatu/performance-handbook/wc-go/cmd"
	"github.com/pkg/profile"
)

func main() {
	defer profile.Start(profile.MemProfile, profile.ProfilePath(".")).Stop()
	cmd.Run()
}
