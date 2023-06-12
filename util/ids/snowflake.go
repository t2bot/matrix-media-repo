package ids

import (
	"errors"
	"os"
	"strconv"

	"github.com/bwmarrin/snowflake"
	"github.com/turt2live/matrix-media-repo/common/config"
)

func GetMachineId() int64 {
	if val, ok := os.LookupEnv("MACHINE_ID"); ok {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			if i == 1023 && !config.Runtime.IsImportProcess {
				panic(errors.New("machine ID 1023 is reserved for use by import process"))
			}
			return i
		}
	}
	return 0
}

var sfnode *snowflake.Node

func makeSnowflake() (*snowflake.Node, error) {
	if sfnode != nil {
		return sfnode, nil
	}
	machineId := GetMachineId()
	node, err := snowflake.NewNode(machineId)
	if err != nil {
		return nil, err
	}
	sfnode = node
	return sfnode, nil
}
