package adb_test

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	adb "github.com/prife/goadb"
	"github.com/stretchr/testify/assert"
)

func newFs() (svr *adb.FileService, err error) {
	d := adbclient.Device(adb.AnyDevice())
	svr, err = d.NewFileService()
	return
}

func TestFileService_PushFile(t *testing.T) {
	fs, err := newFs()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	f := "/Users/wetest/Downloads/RTR4-CN.pdf"
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(t)
	}

	total := float64(info.Size())
	sent := float64(0)
	startTime := time.Now()
	err = fs.PushFile(f, "/sdcard/RTR4-CN.pdf",
		func(n uint64) {
			sent = sent + float64(n)
			percent := float64(sent) / float64(total) * 100
			speedMBPerSecond := float64(sent) * float64(time.Second) / 1024.0 / 1024.0 / (float64(time.Since(startTime)))
			fmt.Printf("push %.02f%% %f Bytes, %.02f MB/s\n", percent, sent, speedMBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileService_PullFile(t *testing.T) {
	fs, err := newFs()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	err = fs.PullFile("/sdcard/WeChatMac.dmg", "WeChatMac.dmg",
		func(total, sent int64, duration time.Duration, status string) {
			percent := float64(sent) / float64(total) * 100
			speedKBPerSecond := float64(sent) * float64(time.Second) / 1024.0 / 1024.0 / float64(duration)
			fmt.Printf("pull %.02f%% %d Bytes / %d, %.02f MB/s\n", percent, sent, total, speedKBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFileService_PushDir(t *testing.T) {
	fs, err := newFs()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	pwd, _ := os.Getwd()

	fmt.Println("workdir: ", pwd)

	err = fs.PushDir(false, "/Users/wetest/workplace/udt/goadb/wire", "/sdcard/test/",
		func(totalFiles, sentFiles uint64, current string, percent, speed float64, err error) {
			if err != nil {
				fmt.Printf("[%d/%d] pushing %s, %%%.2f, err:%s\n", sentFiles, totalFiles, current, percent, err.Error())
			} else {
				fmt.Printf("[%d/%d] pushing %s, %%%.2f, %.02f MB/s\n", sentFiles, totalFiles, current, percent, speed)
			}
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeviceFeatures(t *testing.T) {
	features, err := adbclient.HostFeatures()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("host features: ", features)
	d := adbclient.Device(adb.AnyDevice())
	fmt.Println(d.DeviceFeatures())
}

func TestForwardPort(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	conn, err := d.ForwardPort(50000)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	go func() {
		io.Copy(os.Stdout, conn)
	}()

	for i := 0; i < 10; i++ {
		_, err := conn.Write([]byte("hello, world\n"))
		if err != nil {
			return
		}
		time.Sleep(time.Second * 1)
	}
}

func Test_listAllSubDirs(t *testing.T) {
	gotList, err := adb.ListAllSubDirs("cmd")
	assert.Nil(t, err)

	for _, l := range gotList {
		fmt.Println(l)
	}

	_, err = adb.ListAllSubDirs("non-exsited")
	assert.True(t, os.IsNotExist(err))
}

// $ adb shell mkdir /sdcard/a/ /sdcard/a/b /sdcard/a/b/c
// $ adb shell mkdir /sdcard/a/ /sdcard/a/b /sdcard/a/b/c
// mkdir: '/sdcard/a/': File exists
// mkdir: '/sdcard/a/b': File exists
// mkdir: '/sdcard/a/b/c': File exists
func TestDevice_Mkdirs(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	_, err := d.RunCommand("rm", "-rf", "/sdcard/a")
	assert.Nil(t, err)

	err = d.Mkdirs([]string{"/sdcard/a/", "/sdcard/a/b", "/sdcard/a/b/c"})
	assert.Nil(t, err)
	err = d.Mkdirs([]string{"/sdcard/a/", "/sdcard/a/b", "/sdcard/a/b/c"})
	assert.Nil(t, err)
}

// $ adb shell mkdir /sd/a/ /sd/a/b /sd/a/b/c
// mkdir: '/sd/a/': No such file or directory
// mkdir: '/sd/a/b': No such file or directory
// mkdir: '/sd/a/b/c': No such file or directory
func TestDevice_Mkdirs_NonExsit(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	err := d.Mkdirs([]string{"/sd/a/", "/sd/a/b", "/sd/a/b/c"})
	fmt.Println(err)
	assert.NotNil(t, err)
	lines := strings.Split(err.Error(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			return
		}
		assert.Contains(t, line, "No such file or directory")
	}
}

func TestDevice_Mkdirs_ReadOnly(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	err := d.Mkdirs([]string{"/a", "/b", "/c"})
	fmt.Println(err)
	assert.NotNil(t, err)
	lines := strings.Split(err.Error(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			return
		}
		assert.Contains(t, line, "Read-only file system")
	}
}

func TestDevice_Mkdirs_PermissionDeny(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	err := d.Mkdirs([]string{"/data/a", "/data/b", "/data/c"})
	fmt.Println(err)
	assert.NotNil(t, err)
	lines := strings.Split(err.Error(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			return
		}
		assert.Contains(t, line, "Permission denied")
	}
}

func TestDevice_PushFile(t *testing.T) {
	d := adbclient.Device(adb.AnyDevice())
	err := d.PushFile("/Users/zhongkaizhu/Downloads/test.zip", "/sdcard/test.zip",
		func(totoalSize, sentSize int64, percent, speedMBPerSecond float64) {
			fmt.Printf("%d/%d bytes, %.02f%%, %.02f MB/s\n", sentSize, totoalSize, percent, speedMBPerSecond)
		})
	if err != nil {
		t.Fatal(err)
	}
}
