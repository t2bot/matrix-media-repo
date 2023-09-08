package ids

import (
	"errors"
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

func SetMachineId(id int64) error {
	if err := os.Setenv("MACHINE_ID", strconv.FormatInt(id, 10)); err != nil {
		return err
	}
	sfnode = nil
	if GetMachineId() != id {
		return errors.New("unexpected error setting machine ID")
	}
	if _, err := makeSnowflake(); err != nil {
		return err
	}
	return nil
}
