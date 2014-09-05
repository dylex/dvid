/*
	This file contains code supporting distributed operations such as push/pull with optional
	delimiting using an ROI.
*/

package datastore

import (
	"fmt"
	"strings"
	"time"

	"github.com/janelia-flyem/dvid/dvid"
	"github.com/janelia-flyem/dvid/message"
	"github.com/janelia-flyem/dvid/storage"
)

var (
	NanoPushStart = "PUSH_START"
	NanoPushStop  = "PUSH_STOP"
)

const MaxBatchSize = 1000

func init() {
	message.RegisterOpName(NanoPushStart, handlePush)
}

// Handles a PUSH request, loading repo + data
// TODO -- Be more resilient in the face of errors.  Always read to end of STOP.
func handlePush(s *message.Socket) error {
	if Manager == nil {
		return fmt.Errorf("Can't process pushes when datastore manager not running!")
	}

	// Get the repo metadata
	// TODO -- get additional information indicating origin and push configuration
	repoGob, err := s.ReceiveBinary()
	if err != nil {
		return fmt.Errorf("Error on reading repo metadata: %s\n", err.Error())
	}
	repo := new(repoT)
	if err = repo.GobDecode(repoGob); err != nil {
		return err
	}
	repoID, err := Manager.NewRepoID()
	if err != nil {
		return err
	}
	repo.repoID = repoID

	instanceMap, versionMap, err := repo.remapLocalIDs()
	if err != nil {
		return err
	}

	// Make sure pushed UUID doesn't already exist here.
	duplicateRepo, err := RepoFromUUID(repo.rootID)
	if err != nil {
		return err
	}

	// Get two tiers of storage since we don't know which one will be used for incoming key/value.
	smallStore, err := storage.SmallDataStore()
	if err != nil {
		return err
	}
	smallBatcher, ok := smallStore.(storage.KeyValueBatcher)
	if !ok {
		return fmt.Errorf("Aborting dvid push: Small datastore doesn't support Batch ops")
	}
	bigStore, err := storage.BigDataStore()
	if err != nil {
		return err
	}
	bigBatcher, ok := bigStore.(storage.KeyValueBatcher)
	if !ok {
		return fmt.Errorf("Aborting dvid push: Big datastore doesn't support Batch ops")
	}

	// Store key-value pairs until we get a PUSH STOP.
	// Use a nil storage.Context so we deal with raw keys and don't bother with ConstructKey()
	// transformations using data and version.
	var curStoreType storage.DataStoreType
	var curInstanceID dvid.InstanceID
	var curVersionID dvid.VersionID

	var batchSize int
	var batch storage.Batch

	for {
		msg, err := s.ReceiveMessage()
		if err != nil {
			return fmt.Errorf("Error receiving message on nanomsg socket %s: %s\n", s, err.Error())
		}
		if msg.Type == message.CommandType {
			if msg.Name == NanoPushStop {
				break
			}
			return fmt.Errorf("Expected PUSH STOP.  Got unexpected command instead: %s", msg)
		}
		if msg.Type != message.KeyValueType {
			return fmt.Errorf("Expected key value message within PUSH op.  Got %s instead.", msg.Type)
		}
		if duplicateRepo == nil {
			var flush bool
			if msg.SType != curStoreType {
				flush = true
				curStoreType = msg.SType
			}
			oldInstance, oldVersion, err := storage.KeyToLocalIDs(msg.KV.K)
			if err != nil {
				return err
			}

			// Modify the transmitted key-value to have local instance and version ids.
			newInstanceID, found := instanceMap[oldInstance]
			if !found {
				return fmt.Errorf("Received key with instance id (%d) not present in repo: %v",
					oldInstance, instanceMap)
			}
			newVersionID, found := versionMap[oldVersion]
			if !found {
				return fmt.Errorf("Received key with version id (%d) not present in repo: %v",
					oldVersion, versionMap)
			}

			// Check if we changed instance or version
			if newInstanceID != curInstanceID {
				flush = true
				curInstanceID = newInstanceID
			}
			if newVersionID != curVersionID {
				flush = true
				curVersionID = newVersionID
			}

			// Possibly flush batch
			if flush || batchSize >= MaxBatchSize {
				if batchSize > 0 && batch != nil {
					if err := batch.Commit(); err != nil {
						return err
					}
					batchSize = 0
				}
				switch curStoreType {
				case storage.SmallData:
					batch = smallBatcher.NewBatch(nil)
				case storage.BigData:
					batch = bigBatcher.NewBatch(nil)
				}
			}

			// Store the updated key-value
			if err = storage.UpdateDataContextKey(msg.KV.K, curInstanceID, curVersionID); err != nil {
				return fmt.Errorf("Unable to update DataContext key %v", msg.KV.K)
			}
			batch.Put(msg.KV.K, msg.KV.V)
			batchSize++
		}
	}

	if duplicateRepo != nil {
		return fmt.Errorf("Pushed repo %s already exists in this DVID server", repo.rootID)
	}

	// Make sure any partial batch is saved.
	if batchSize > 0 {
		if err := batch.Commit(); err != nil {
			return err
		}
	}

	// Add this repo to current DVID server
	return Manager.AddRepo(repo)
}

func Push(repo Repo, target string, config dvid.Config) error {
	// Get the push configuration
	roiname, err := getROI(config)
	if err != nil {
		return err
	}
	data, err := getDataInstances(repo, config)
	if err != nil {
		return err
	}

	// Establish connection with target, which may be itself
	s, err := message.NewPushSocket(target)
	if err != nil {
		return fmt.Errorf("Unable to create new push socket: %s", err.Error())
	}

	// Send PUSH command start
	if err = s.SendCommand(NanoPushStart); err != nil {
		return err
	}

	// Send the repo metadata
	// TODO -- add additional information indicating origin and push configuration
	dvid.Debugf("Sending repo %s data to %q\n", repo.RootUUID(), target)
	repoSerialization, err := repo.GobEncode()
	if err != nil {
		return err
	}
	if err = s.SendBinary("repo", repoSerialization); err != nil {
		return err
	}

	// For each data instance, send the data delimited by the roi
	for _, instance := range data {
		if err := instance.Send(s, roiname); err != nil {
			return err
		}
	}

	// Send PUSH command end
	if err = s.SendCommand(NanoPushStop); err != nil {
		return err
	}

	// Close the connection.
	time.Sleep(1 * time.Second)
	if err = s.Close(); err != nil {
		return err
	}
	return nil
}

func Pull(repo Repo, target string, config dvid.Config) error {
	// To Pull() we initiate a push from target.
	// It's up to target whether it will push or not.
	return nil
}

// Return roi name or empty string
func getROI(config dvid.Config) (string, error) {
	roiname, found, err := config.GetString("roi")
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return roiname, nil
}

// Return all data instances or just those selected in configuration.
func getDataInstances(repo Repo, config dvid.Config) ([]DataService, error) {
	// Since we can have names separated by commas, split them
	namesString, found, err := config.GetString("data")
	if err != nil {
		return nil, err
	}
	datanames := strings.Split(namesString, ",")

	var datalist []DataService
	if !found || len(datanames) == 0 {
		// use all data instances
		data, err := repo.GetAllData()
		if err != nil {
			return nil, err
		}
		for _, dataservice := range data {
			datalist = append(datalist, dataservice)
		}
		return datalist, nil
	}
	// use only those data instances given
	for _, name := range datanames {
		dataservice, err := repo.GetDataByName(dvid.DataString(name))
		if err != nil {
			return nil, err
		}
		datalist = append(datalist, dataservice)
	}
	return datalist, nil
}
