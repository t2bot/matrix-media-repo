package import_cmdline

import (
	"fmt"
	"os"

	"github.com/t2bot/matrix-media-repo/common/version"
	"github.com/t2bot/matrix-media-repo/util/ids"
	"golang.org/x/term"
)

func AskMachineId() {
	fmt.Printf("The importer runs as a MMR worker and needs to have a dedicated MACHINE_ID. See https://docs.t2bot.io/matrix-media-repo/%s/deployment/horizontal_scaling for details on what a MACHINE_ID is.", version.DocsVersion)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Println("Please specify a MACHINE_ID environment variable.")
		os.Exit(2)
		return // for good measure
	}
	fmt.Println("If you don't use horizontal scaling, you can use '1' as the machine ID. Otherwise, please enter an unused machine ID in your environment.")
	fmt.Printf("Machine ID: ")
	var machineId int64
	if _, err := fmt.Scanf("%d", &machineId); err != nil {
		panic(err)
	}
	if err := ids.SetMachineId(machineId); err != nil {
		panic(err)
	}
}
