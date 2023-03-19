package main

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"

	"github.com/beeper/bridge-manager/bridgeconfig"
)

var configCommand = &cli.Command{
	Name:      "config",
	Aliases:   []string{"c"},
	Usage:     "Generate a config for an official Beeper bridge",
	ArgsUsage: "BRIDGE",
	Before:    RequiresAuth,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "address",
			Aliases: []string{"a"},
			EnvVars: []string{"BEEPER_BRIDGE_ADDRESS"},
			Usage:   "Optionally, a https address where the Beeper server can push events.\nWhen omitted, the server will expect the bridge to connect with a websocket to receive events.",
		},
		&cli.StringFlag{
			Name:    "listen",
			Aliases: []string{"l"},
			EnvVars: []string{"BEEPER_BRIDGE_LISTEN_ADDRESS"},
			Usage:   "IP and port where the bridge should listen. Only relevant when address is specified.",
		},
		&cli.StringFlag{
			Name:    "type",
			Aliases: []string{"t"},
			EnvVars: []string{"BEEPER_BRIDGE_TYPE"},
			Usage:   "The type of bridge being registered.",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   "-",
			EnvVars: []string{"BEEPER_BRIDGE_CONFIG_FILE"},
			Usage:   "Path to save generated config file to.",
		},
	},
	Action: generateBridgeConfig,
}

var websocketBridges = map[string]bool{
	"imessage": true,
}

func generateBridgeConfig(ctx *cli.Context) error {
	if ctx.NArg() == 0 {
		return UserError{"You must specify a bridge to generate a config for"}
	} else if ctx.NArg() > 1 {
		return UserError{"Too many arguments specified (flags must come before arguments)"}
	}
	bridge := ctx.Args().Get(0)
	if !allowedBridgeRegex.MatchString(bridge) {
		return UserError{"Invalid bridge name"}
	}
	bridgeType := ctx.String("type")
	if bridgeType == "" {
		bridgeType = officialBridges[bridge]
		if bridgeType == "" {
			bridgeType = bridge
		}
	}
	if !bridgeconfig.IsSupported(bridgeType) {
		_, _ = fmt.Fprintln(os.Stderr, color.YellowString("Unsupported bridge type"), color.CyanString(bridgeType))
		err := survey.AskOne(&survey.Select{
			Message: "Select bridge type:",
			Options: bridgeconfig.SupportedBridges,
		}, &bridgeType)
		if err != nil {
			return err
		}
	}
	isWebsocket := ctx.String("address") == ""
	if !isWebsocket && ctx.String("listen") == "" {
		return UserError{"Both --listen and --address must be provided when not using websocket mode"}
	} else if isWebsocket && !websocketBridges[bridgeType] {
		return UserError{fmt.Sprintf("%s doesn't support websockets yet, please provide --address and --listen", bridgeType)}
	}
	reg, err := doRegisterBridge(ctx, bridge, false)
	if err != nil {
		return err
	}
	var listenAddr string
	var listenPort uint16
	if !isWebsocket {
		_, err = fmt.Sscanf(ctx.String("listen"), "%s:%d", &listenAddr, &listenPort)
		if err != nil {
			return fmt.Errorf("failed to parse listen address: %w", err)
		}
	}

	cfg, err := bridgeconfig.Generate(bridgeType, bridgeconfig.Params{
		HungryAddress: reg.HomeserverURL,
		BeeperDomain:  ctx.String("homeserver"),
		Websocket:     reg.Registration.URL == "websocket",
		ListenAddr:    listenAddr,
		ListenPort:    listenPort,
		AppserviceID:  reg.Registration.ID,
		ASToken:       reg.Registration.AppToken,
		HSToken:       reg.Registration.ServerToken,
		BridgeName:    bridge,
		UserID:        reg.YourUserID,
		Params:        nil,
	})
	if err != nil {
		return err
	}
	return doOutputFile(ctx, "Config", cfg)
}
