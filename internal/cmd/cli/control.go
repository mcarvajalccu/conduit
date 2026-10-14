// Copyright 2026. Triad National Security, LLC. All rights reserved.

package clicmd

import (
	"fmt"
	"os"

	proto "github.com/lanl/conduit/api"
	"github.com/lanl/conduit/defaults"
	"github.com/lanl/conduit/internal/cli/client"
	"github.com/lanl/conduit/internal/cli/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
)

// Escape codes for colored server status output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorBlue   = "\033[34m"
	ColorYellow = "\033[33m"
)

var (
	serverCmdVerbose bool
	serverCmdJSON    bool
)

var serverCmd = &cobra.Command{
	Use:    "server",
	Short:  "control the conduit server state",
	Long:   `This subcommand allows an admin to control the conduit server state`,
	Hidden: true,
}

var serverStartCmd = &cobra.Command{
	Use:    "start",
	Short:  "starts the conduit server",
	Long:   `This subcommand starts the conduit server so it will start progressing transfers again`,
	Hidden: false,
	Run: func(cmd *cobra.Command, args []string) {
		logger := logrus.New()
		if debug {
			logger.SetLevel(logrus.DebugLevel)
			logger.Debugf("loaded cli config from: %v", viper.ConfigFileUsed())
		}

		clientCertKeyBundle, err := cmd.Flags().GetString("cert-key-bundle")
		if err != nil {
			fmt.Printf("failed to get cert-key-bundle flag: %v\n", err)
			os.Exit(1)
		}
		clientCert, clientKey, err := util.GetUserCertAndKey(viper.GetString(defaults.ConfigClientCertKey), viper.GetString(defaults.ConfigClientKeyKey), clientCertKeyBundle, defaults.DefaultBundlePath)
		if err != nil {
			fmt.Printf("failed to get client cert and key: %v\n", err)
			os.Exit(1)
		}
		logger.Debugf("using user cert [%v] and key [%v]", clientCert, clientKey)

		client, err := client.NewClient(logger, quiet, clientCert, clientKey)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		resp, err := client.ServerControl(proto.ServerControlAction_SERVER_CONTROL_START)
		if err != nil {
			fmt.Printf("server start request failed: %v\n", err)
			os.Exit(1)
		} else {
			fmt.Printf("successfully started conduit server. current state: %s\n", resp.GetServerState())
		}
	},
}

var serverDrainCmd = &cobra.Command{
	Use:    "drain",
	Short:  "drains the conduit server",
	Long:   `This subcommand drains the conduit server so it will progress currently queued transfers but will not progress any that are newly submitted. Note that this only works if all conduit instances for a cluster are draining. Use 'stop' to safely stop a single conduit instance`,
	Hidden: false,
	Run: func(cmd *cobra.Command, args []string) {
		logger := logrus.New()
		if debug {
			logger.SetLevel(logrus.DebugLevel)
		}

		clientCertKeyBundle, err := cmd.Flags().GetString("cert-key-bundle")
		if err != nil {
			fmt.Printf("failed to get cert-key-bundle flag: %v\n", err)
			os.Exit(1)
		}
		clientCert, clientKey, err := util.GetUserCertAndKey(viper.GetString(defaults.ConfigClientCertKey), viper.GetString(defaults.ConfigClientKeyKey), clientCertKeyBundle, defaults.DefaultBundlePath)
		if err != nil {
			fmt.Printf("failed to get client cert and key: %v\n", err)
			os.Exit(1)
		}
		logger.Debugf("using user cert [%v] and key [%v]", clientCert, clientKey)

		client, err := client.NewClient(logger, quiet, clientCert, clientKey)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		resp, err := client.ServerControl(proto.ServerControlAction_SERVER_CONTROL_DRAIN)
		if err != nil {
			fmt.Printf("server drain request failed: %v\n", err)
			os.Exit(1)
		} else {
			fmt.Printf("successfully started draining conduit server. current state: %s\n", resp.GetServerState())
		}
	},
}

var serverStatusCmd = &cobra.Command{
	Use:    "status",
	Short:  "get conduit server state",
	Long:   `This subcommand will retrieve the current state of the conduit server`,
	Hidden: false,
	Run: func(cmd *cobra.Command, args []string) {
		logger := logrus.New()
		if debug {
			logger.SetLevel(logrus.DebugLevel)
		}

		clientCert := viper.GetString(defaults.ConfigClientCertKey)
		clientKey := viper.GetString(defaults.ConfigClientKeyKey)

		client, err := client.NewClient(logger, quiet, clientCert, clientKey)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if serverCmdJSON {
			var stateJSON string
			resp, err := client.ServerControl(proto.ServerControlAction_SERVER_CONTROL_STATUS)
			if err != nil {
				stateJSON = fmt.Sprintf(`"server_state_error": %q`, err.Error())
			} else {
				stateJSON = fmt.Sprintf(`"server_state": %q`, resp.GetServerState().String())
			}

			var schedJSON string
			schedFailed := false
			schedResp, err := client.SchedulerInfo()
			if err != nil {
				schedFailed = true
				schedJSON = fmt.Sprintf("\n  \"scheduler_error\": %q\n", err.Error())
			} else {
				mo := protojson.MarshalOptions{
					Multiline:       true,
					EmitUnpopulated: true,
				}

				srBytes, err := mo.Marshal(schedResp)
				if err != nil {
					schedJSON = fmt.Sprintf("\n  \"scheduler_error\": %q\n", "failed to marshal scheduler info: "+err.Error())
				} else {
					schedJSON = string(srBytes[1 : len(srBytes)-1])
				}
			}

			fmt.Printf("{\n  %s,%s}\n", stateJSON, schedJSON)

			if schedFailed {
				os.Exit(1)
			}

		} else {

			fmt.Println("Conduit Server")
			fmt.Println("──────────────")

			resp, err := client.ServerControl(proto.ServerControlAction_SERVER_CONTROL_STATUS)
			if err != nil {
				fmt.Printf("State        "+ColorRed+"server status request failed: %v\n"+ColorReset, err)
			} else {
				serv_state := resp.GetServerState()
				var msgColor string
				switch serv_state {
				case proto.ServerState_SERVER_RUNNING:
					msgColor = ColorGreen
				case proto.ServerState_SERVER_STARTING,
					proto.ServerState_SERVER_STOPPING,
					proto.ServerState_SERVER_DRAIN_INIT,
					proto.ServerState_SERVER_DRAINING,
					proto.ServerState_SERVER_UNKNOWN:
					msgColor = ColorYellow
				case proto.ServerState_SERVER_ERROR:
					msgColor = ColorRed
				case proto.ServerState_SERVER_STOPPED:
					msgColor = ColorReset
				}

				fmt.Printf("State        "+msgColor+"%s\n"+ColorReset, resp.GetServerState())
			}

			schedResp, err := client.SchedulerInfo()
			if err != nil {
				fmt.Printf("Schedulers   "+ColorRed+"server scheduler request failed: %v\n"+ColorReset, err)
				os.Exit(1)
			} else {
				schedulers := schedResp.GetSchedulers()
				fmt.Printf("Schedulers   %d\n", len(schedulers))

				if serverCmdVerbose {
					fmt.Println("")
					for schedID, schedStatus := range schedulers {
						fmt.Printf("\nScheduler "+ColorBlue+"%s\n"+ColorReset, schedID)
						fmt.Println("───────────────────────────────────────────────────────────────────────────────────────────────────────")
						fmt.Printf("Queue Size   %d\n\n", schedStatus.GetQueueSize())

						fmt.Println("NODE                                 JOBS     AVAILABLE MEMORY     JOBS VERSION     RUNNER VERSION")
						for nodeID, nodeInfo := range schedStatus.Nodes {
							fmt.Printf("%-35.35s  %-3d      %-6.2f GiB           %-3d              %s\n", nodeID, len(nodeInfo.GetJobs()), (float64(nodeInfo.GetAvailableMemory()))/(1024*1024*1024), nodeInfo.GetJobsVersion(), nodeInfo.GetRunnerVersion())
						}

						fmt.Println("\nJobs")
						fmt.Println("───────────────────────────────────────────────────────────────────────────────────────────────────────")
						fmt.Println("NODE                                 JOB ID")
						for nodeID, nodeInfo := range schedStatus.Nodes {
							for jobID := range nodeInfo.GetJobs() {
								fmt.Printf("%-35.35s  %s\n", nodeID, jobID)
							}
						}

						fmt.Println("\nQueue")
						fmt.Println("───────────────────────────────────────────────────────────────────────────────────────────────────────")
						fmt.Println("POSITION                             TRANSFER / JOB")
						for pos, job := range schedStatus.GetQueue() {
							fmt.Printf("%-35d  %s\n", pos+1, job.GetJobID())
						}
						fmt.Printf("\n\n")
					}
				} else {
					fmt.Println("\n\nSchedulers")
					fmt.Println("─────────────────────────────────────────────────")
					fmt.Println("SCHEDULER     NODES    JOBS    QUEUED")
					for schedID, schedStatus := range schedulers {
						schedJobs := 0
						for _, nodeInfo := range schedStatus.Nodes {
							schedJobs += len(nodeInfo.GetJobs())
						}
						fmt.Printf("%s      %-3d      %-3d     %d\n", schedID[:8], len(schedStatus.Nodes), schedJobs, schedStatus.GetQueueSize())
					}

					fmt.Println("\n\nNodes")
					fmt.Println("───────────────────────────────────────────────────────────────────────────────")
					fmt.Println("NODE                   JOBS     AVAILABLE MEMORY     RUNNER VERSION")
					for _, schedStatus := range schedulers {
						for nodeID, nodeInfo := range schedStatus.Nodes {
							fmt.Printf("%-16.16s       %-3d      %-6.2f GiB           %s\n", nodeID, len(nodeInfo.GetJobs()), (float64(nodeInfo.GetAvailableMemory()))/(1024*1024*1024), nodeInfo.GetRunnerVersion())
						}
					}
				}
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverDrainCmd)
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStatusCmd)

	serverStatusCmd.Flags().BoolVar(&serverCmdVerbose, "verbose", false, "Display verbose information about the conduit server state")
	serverStatusCmd.Flags().BoolVar(&serverCmdJSON, "json", false, "Display information in JSON format")
}
