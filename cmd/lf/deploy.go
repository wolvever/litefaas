package main

func cmdDeploy(args []string) error {
	gw, tok, cfgDir, rest := gatewayFlags(args)
	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	c, err := resolveClient(gw, tok, cfgDir)
	if err != nil {
		return err
	}
	return registerAndDeploy(c, dir)
}
