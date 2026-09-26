package main

import (
	"fmt"
	"os"

	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/token"
)

func cmdToken(args []string) error {
	gw, tokFlag, dir, _ := gatewayFlags(args)
	_ = gw
	if dir == "" {
		dir = config.DefaultDir()
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	_, ctx, err := cfg.CurrentContext()
	if err != nil {
		return err
	}
	source := "none"
	tok := token.ResolveClient(tokFlag, os.Getenv("LITEFAAS_TOKEN"), ctx.Token, dir)
	switch {
	case tokFlag != "":
		source = "flag"
	case os.Getenv("LITEFAAS_TOKEN") != "":
		source = "env"
	case ctx.Token != "":
		source = "context"
	case tok != "":
		source = "file:" + token.Path(dir)
	}
	if tok == "" {
		fmt.Println("token= source=none")
		return nil
	}
	fmt.Printf("token=%s source=%s\n", tok, source)
	return nil
}
