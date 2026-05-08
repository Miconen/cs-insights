package main

import (
	"fmt"
	"os"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

func main() {
	f, _ := os.Open("./test-demo.dem")
	defer f.Close()

	cfg := demoinfocs.DefaultParserConfig
	cfg.IgnoreErrBombsiteIndexNotFound = true
	cfg.IgnorePacketEntitiesPanic = true
	
	p := demoinfocs.NewParserWithConfig(f, cfg)
	defer p.Close()

	var count int
	p.RegisterEventHandler(func(e events.WeaponFire) {
		if count > 0 {
			return
		}
		count++
		for _, prop := range e.Shooter.Entity.Properties() {
			if prop.Property() != nil {
				name := prop.Property().Name()
				if name != "" && (name[0] == 'm' || name[0] == 'v') {
					fmt.Println(name)
				}
			}
		}
	})

	p.ParseNextFrame()
	for count == 0 {
		more, _ := p.ParseNextFrame()
		if !more {
			break
		}
	}
}
