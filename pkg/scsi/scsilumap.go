/*
Copyright 2016 The GoStor Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scsi

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/openebs/gotgt/pkg/api"
	"github.com/openebs/gotgt/pkg/config"
)

type BackendType string

type SCSILUMap struct {
	mutex sync.RWMutex
	// use UUID as the key for all LUs
	AllDevices api.LUNMap
	// use target name as the key for target's LUN map
	TargetsLUNMap map[string]api.LUNMap
	TargetsBSMap  map[string]api.IOs /* use target name as the key for target's Backing Store (temp) */
}

var globalSCSILUMap = SCSILUMap{
	AllDevices:    make(api.LUNMap),
	TargetsLUNMap: make(map[string]api.LUNMap),
	TargetsBSMap:  make(map[string]api.IOs),
}

func mappingLUN(deviceID uint64, lun uint64, target string) {

	device := globalSCSILUMap.AllDevices[deviceID]
	lunMap := globalSCSILUMap.TargetsLUNMap[target]
	if lunMap == nil {
		globalSCSILUMap.TargetsLUNMap[target] = make(api.LUNMap)
		lunMap = globalSCSILUMap.TargetsLUNMap[target]
	}
	lunMap[lun] = device
}

func GetLU(tgtName string, LUN uint64) *api.SCSILu {
	globalSCSILUMap.mutex.RLock()
	defer globalSCSILUMap.mutex.RUnlock()

	lunMap := globalSCSILUMap.TargetsLUNMap[tgtName]
	lun := lunMap[LUN]

	return lun
}

func GetTargetLUNMap(tgtName string) api.LUNMap {
	globalSCSILUMap.mutex.RLock()
	defer globalSCSILUMap.mutex.RUnlock()

	lunMap := globalSCSILUMap.TargetsLUNMap[tgtName]
	return lunMap
}

func GetTargetBSMap(tgtName string) api.IOs {
	/* TODO check for lock held by caller
	globalSCSILUMap.mutex.RLock()
	defer globalSCSILUMap.mutex.RUnlock()*/

	lunMap := globalSCSILUMap.TargetsBSMap[tgtName]
	return lunMap
}

func InitSCSILUMap(config *config.Config) error {
	var simpleOp *SCSISimpleReservationOperator
	var ok bool
	globalSCSILUMap.mutex.Lock()
	defer globalSCSILUMap.mutex.Unlock()

	for _, bs := range config.Storages {
		lu, err := NewSCSILu(bs.DeviceID, bs.Path, bs.Online)
		if err != nil {
			return fmt.Errorf("Init SCSI LU map error: %v", err)
		}
		globalSCSILUMap.AllDevices[bs.DeviceID] = lu
	}

	for tgtName, tgt := range config.ISCSITargets {
		for lunstr, deviceID := range tgt.LUNs {
			lun, err := strconv.ParseUint(lunstr, 10, 64)
			if err != nil {
				return fmt.Errorf("LU Number must be a number")
			}
			mappingLUN(deviceID, lun, tgtName)
			// Init SCSISimpleReservationOperator
			op := GetSCSIReservationOperator()
			if simpleOp, ok = op.(*SCSISimpleReservationOperator); ok {
				simpleOp.InitLUReservation(tgtName, deviceID)
			}
		}
	}
	return nil
}

func InitSCSILUMapEx(tgtName, devpath string, deviceID, lun, size, sectorSize uint64, bs api.IOs) error {
	globalSCSILUMap.mutex.Lock()
	defer globalSCSILUMap.mutex.Unlock()

	lu, err := NewSCSILu(deviceID, "RemBs:"+devpath, true)
	if err != nil {
		return errors.New("Init SCSI LU map error.")
	}
	globalSCSILUMap.AllDevices[deviceID] = lu

	/* TODO workaround for now */
	globalSCSILUMap.TargetsBSMap[tgtName] = bs
	lu.Size = size
	err = lu.Storage.Open(lu, tgtName)

	/*lu.Storage.Size(lu)*/
	lu.DeviceProtocol.InitLu(lu)

	mappingLUN(deviceID, lun, tgtName)

	return nil
}
