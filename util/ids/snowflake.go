package ids

import (
	"os"
	"strconv"

	"github.com/bwmarrin/snowflake"
)

func GetMachineId() int64 {
	if val, ok := os.LookupEnv("MACHINE_ID"); ok {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
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
