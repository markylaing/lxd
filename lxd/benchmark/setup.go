package benchmark

import (
	"context"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/instance/instancetype"
	"github.com/canonical/lxd/shared/osarch"
	"golang.org/x/exp/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

const (
	nClusterMembers      = 5
	nProjects            = 5
	nProfilesPerProject  = 20
	nInstancesPerProject = 100
	nProfilesPerInstance = 3
	nProfileConfigKeys   = 50
	nConfigsPerProfile   = 3
)

var (
	lookupConfig      [2]string
	lookupProject     string
	allProfileConfigs = make([][2]string, 0, nProfileConfigKeys)
	clusterDB         *db.Cluster
)

func projectName(i int) string {
	return "project_" + strconv.Itoa(i)
}

func clusterMemberName(i int) string {
	return "member_" + strconv.Itoa(i)
}

func clusterMemberAddress(i int) string {
	return "192.0.2." + strconv.Itoa(i)
}

func instanceName(i int) string {
	return "instance_" + strconv.Itoa(i)
}

func profileName(i int) string {
	return "profile_" + strconv.Itoa(i)
}

func instanceConfig(p instancetype.Type) map[string]string {
	var conf map[string]string
	switch p {
	case instancetype.Container:
		conf = map[string]string{
			"image.architecture":              "amd64",
			"image.description":               "ubuntu 24.04 LTS amd64 (release) (20250403)",
			"image.label":                     "release",
			"image.os":                        "ubuntu",
			"image.release":                   "noble",
			"image.serial":                    "20250403",
			"image.type":                      "squashfs",
			"image.version":                   "24.04",
			"limits.cpu":                      "2",
			"limits.memory":                   "2GiB",
			"volatile.base_image":             "9f684552788a49591b1336a37e943296d346e345252cade971377a8d4df4e9c7",
			"volatile.cloud-init.instance-id": "cdb2464b-a50c-4d53-989d-08710b7569a1",
			"volatile.eth0.host_name":         "veth0434642c",
			"volatile.eth0.hwaddr":            "00:16:3e:c5:a3:6b",
			"volatile.idmap.base":             "0",
			"volatile.idmap.current":          `[{"Isuid":true,"Isgid":false,"Hostid":1000000,"Nsid":0,"Maprange":1000000000},{"Isuid":false,"Isgid":true,"Hostid":1000000,"Nsid":0,"Maprange":1000000000}]`,
			"volatile.idmap.next":             `[{"Isuid":true,"Isgid":false,"Hostid":1000000,"Nsid":0,"Maprange":1000000000},{"Isuid":false,"Isgid":true,"Hostid":1000000,"Nsid":0,"Maprange":1000000000}]`,
			"volatile.last_state.idmap":       "[]",
			"volatile.last_state.power":       "RUNNING",
			"volatile.uuid":                   "12a16c87-5480-45fc-8f52-c71917a72305",
			"volatile.uuid.generation":        "12a16c87-5480-45fc-8f52-c71917a72305",
		}
	case instancetype.VM:
		conf = map[string]string{
			"image.architecture":              "amd64",
			"image.description":               "ubuntu 24.04 LTS amd64 (release) (20250403)",
			"image.label":                     "release",
			"image.os":                        "ubuntu",
			"image.release":                   "noble",
			"image.serial":                    "20250403",
			"image.type":                      "disk1.img",
			"image.version":                   "24.04",
			"limits.cpu":                      "2",
			"limits.memory":                   "4GiB",
			"volatile.base_image":             "97121cfbcd9e9f2b0d0e15f049ad2b45186cecb922a1351368754db20215915a",
			"volatile.cloud-init.instance-id": "a820fd29-b492-420b-a7e6-ad24ddacf90e",
			"volatile.eth0.host_name":         "tap2a3c7553",
			"volatile.eth0.hwaddr":            "00:16:3e:f7:06:c4",
			"volatile.last_state.power":       "RUNNING",
			"volatile.uuid":                   "64205494-0207-48f9-a338-7366f8c5590f",
			"volatile.uuid.generation":        "64205494-0207-48f9-a338-7366f8c5590f",
			"volatile.vsock_id":               "3989549452",
		}
	default:
		panic("Invalid instance type")
	}

	// Grab a few configs randomly from the profiles
	for k, v := range profileConfig(5) {
		conf[k] = v
	}

	return conf
}

func profileConfig(n int) map[string]string {
	permIdx := rand.Perm(len(allProfileConfigs))
	config := make(map[string]string, n)
	for i := 0; i < n; i++ {
		config[allProfileConfigs[permIdx[i]][0]] = allProfileConfigs[permIdx[i]][1]
	}

	return config
}

func init() {
	testing.Init()
	handleErr := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	for i := 1; i <= nProfileConfigKeys; i++ {
		idxStr := strconv.Itoa(i)
		allProfileConfigs = append(allProfileConfigs, [][2]string{{"user.foo_" + idxStr, "bar_" + idxStr}}...)
	}

	lookupConfig = allProfileConfigs[rand.Intn(nProfileConfigKeys)]
	lookupProject = projectName(rand.Intn(nProjects) + 1)

	_ = os.RemoveAll("test.db")
	clusterDB, _ = db.NewTestClusterWithDataSource(&testing.T{}, "file:test.db")

	err := clusterDB.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
		for i := 1; i <= nClusterMembers; i++ {
			_, err := tx.CreateNode(clusterMemberName(i), clusterMemberAddress(i))
			handleErr(err)
		}

		for i := 1; i <= nProjects; i++ {
			projName := projectName(i)
			_, err := cluster.CreateProject(ctx, tx.Tx(), cluster.Project{
				Name: projName,
			})
			handleErr(err)

			profileIDs := make([]int64, 0, nProfilesPerProject)
			for j := 1; j <= nProfilesPerProject; j++ {
				profileID, err := cluster.CreateProfile(ctx, tx.Tx(), cluster.Profile{
					Project:     projectName(i),
					Name:        profileName(j),
					Description: "Test profile " + strconv.Itoa(j),
				})
				handleErr(err)
				profileIDs = append(profileIDs, profileID)

				err = cluster.CreateProfileConfig(ctx, tx.Tx(), profileID, profileConfig(nConfigsPerProfile))
				handleErr(err)
			}

			for j := 1; j <= nInstancesPerProject; j++ {
				clusterMemberID := (j % nClusterMembers) + 1
				instanceType := instancetype.Type(j % 2)
				instanceID, err := cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
					Project:      projName,
					Name:         instanceName(j),
					Node:         clusterMemberName(clusterMemberID),
					Type:         instanceType,
					Snapshot:     false,
					Architecture: osarch.ARCH_64BIT_INTEL_X86,
					Ephemeral:    false,
					CreationDate: time.Now(),
					Stateful:     false,
					Description:  "Test instance " + strconv.Itoa(j),
				})
				handleErr(err)

				err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, instanceConfig(instanceType))
				handleErr(err)

				permIdx := rand.Perm(nProfilesPerInstance)
				for k := 1; k <= nProfilesPerInstance; k++ {
					err = cluster.CreateInstanceProfiles(ctx, tx.Tx(), []cluster.InstanceProfile{
						{
							ApplyOrder: k,
							InstanceID: int(instanceID),
							ProfileID:  int(profileIDs[permIdx[k-1]]),
						},
					})
					handleErr(err)
				}
			}
		}

		return nil
	})
	handleErr(err)
}
