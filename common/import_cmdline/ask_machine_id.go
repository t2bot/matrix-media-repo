package import_cmdline

import (
	"fmt"
	"os"

	"github.com/turt2live/matrix-media-repo/util/ids"
	"golang.org/x/term"
)

func AskMachineId() {
	fmt.Println("The importer runs as a MMR worker and needs to have a dedicated MACHINE_ID. See https://docs.t2bot.io/matrix-media-repo/deployment/horizontal_scaling.html for details on what a MACHINE_ID is.")
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
