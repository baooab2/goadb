package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	adb "DomaphoneS-Next/backend/goadb"
	"DomaphoneS-Next/backend/goadb/wire"

	"github.com/alecthomas/kingpin/v2"
	"github.com/cheggaaa/pb"
)

const StdIoFilename = "-"

var (
	serial = kingpin.Flag("serial",
		"Connect to device by serial number.").
		Short('s').
		String()

	shellCommand = kingpin.Command("shell",
		"Run a shell command on the device.")
	shellCommandArg = shellCommand.Arg("command",
		"Command to run on device.").
		Strings()
	psCommand = kingpin.Command("ps",
		"List processes.")
	devicesCommand = kingpin.Command("devices",
		"List devices.")
	devicesLongFlag = devicesCommand.Flag("long",
		"Include extra detail about devices.").
		Short('l').
		Bool()

	pullCommand = kingpin.Command("pull",
		"Pull a file from the device.")
	pullProgressFlag = pullCommand.Flag("progress",
		"Show progress.").
		Short('p').
		Bool()
	pullRemoteArg = pullCommand.Arg("remote",
		"Path of source file on device.").
		Required().
		String()
	pullLocalArg = pullCommand.Arg("local",
		"Path of destination file. If -, will write to stdout.").
		String()
	pushCommand = kingpin.Command("push",
		"Push a file to the device.")
	pushProgressFlag = pushCommand.Flag("progress",
		"Show progress.").
		Short('p').
		Bool()
	pushLocalArg = pushCommand.Arg("local",
		"Path of source file. If -, will read from stdin.").
		Required().
		String()
	pushRemoteArg = pushCommand.Arg("remote",
		"Path of destination file on device.").
		Required().
		String()
	pushCommand2 = kingpin.Command("push2",
		"Push to the device.")
	pushLocalArg2 = pushCommand2.Arg("local",
		"Path of source file. If -, will read from stdin.").
		Required().
		String()
	pushRemoteArg2 = pushCommand2.Arg("remote",
		"Path of destination file on device.").
		Required().
		String()
)

var client *adb.Adb

func main() {
	var exitCode int

	var err error
	client, err = adb.NewWithConfig(adb.ServerConfig{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	switch kingpin.Parse() {
	case "devices":
		exitCode = listDevices(*devicesLongFlag)
	case "shell":
		exitCode = runShellCommand(*shellCommandArg, parseDevice())
	case "ps":
		exitCode = ps(parseDevice())
	case "pull":
		exitCode = pull(*pullProgressFlag, *pullRemoteArg, *pullLocalArg, parseDevice())
	case "push":
		exitCode = push(*pushProgressFlag, *pushLocalArg, *pushRemoteArg, parseDevice())
	case "push2":
		exitCode = push2(parseDevice(), *pushLocalArg2, *pushRemoteArg2)
	}

	os.Exit(exitCode)
}

func parseDevice() adb.DeviceDescriptor {
	if *serial != "" {
		return adb.DeviceWithSerial(*serial)
	}

	return adb.AnyDevice()
}

func listDevices(long bool) int {
	//client := adb.New(server)
	devices, err := client.ListDevices()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}

	for _, device := range devices {
		fmt.Printf("%s\t %s ", device.Serial, device.State)
		if device.Usb != "" {
			fmt.Printf("usb:%s ", device.Usb)
		}

		if device.Product != "" {
			fmt.Printf("product:%s ", device.Product)
		}

		if device.Model != "" {
			fmt.Printf("model:%s ", device.Model)
		}

		if device.DeviceInfo != "" {
			fmt.Printf("device:%s ", device.DeviceInfo)
		}

		if device.TransportID != 0 {
			fmt.Printf("transport_id:%d ", device.TransportID)
		}
		fmt.Printf("\n")
	}

	return 0
}

func runShellCommand(commandAndArgs []string, device adb.DeviceDescriptor) int {
	if len(commandAndArgs) == 0 {
		fmt.Fprintln(os.Stderr, "error: no command")
		kingpin.Usage()
		return 1
	}

	command := commandAndArgs[0]
	var args []string

	if len(commandAndArgs) > 1 {
		args = commandAndArgs[1:]
	}

	client := client.Device(device)
	reader, err := client.RunCommand(command, args...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	_ = reader
	fmt.Printf("%s\n", reader)
	fmt.Println("--------------")
	fmt.Println(hex.Dump(reader))
	// defer reader.Close()
	// io.Copy(os.Stdout, reader)
	return 0
}

func ps(device adb.DeviceDescriptor) int {
	client := client.Device(device)
	list, err := client.ListProcesses(nil)
	if err != nil {
		panic(err)
		return 1
	}
	for _, name := range list {
		fmt.Printf("%-12s%6d%49s%s\n", name.Uid, name.Pid, " ", name.Name)
	}
	return 0
}

func pull(showProgress bool, remotePath, localPath string, device adb.DeviceDescriptor) int {
	if remotePath == "" {
		fmt.Fprintln(os.Stderr, "error: must specify remote file")
		kingpin.Usage()
		return 1
	}

	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}

	client := client.Device(device)

	info, err := client.Stat(remotePath)
	if errors.Is(err, wire.ErrFileNoExist) {
		fmt.Fprintln(os.Stderr, "remote file does not exist:", remotePath)
		return 1
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error reading remote file %s: %s\n", remotePath, err)
		return 1
	}

	sc, remoteFile, err := client.OpenFileReader(remotePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening remote file %s: %v\n", remotePath, err)
		return 1
	}
	defer sc.Close()

	var localFile io.WriteCloser
	if localPath == StdIoFilename {
		localFile = os.Stdout
	} else {
		localFile, err = os.Create(localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening local file %s: %s\n", localPath, err)
			return 1
		}
	}
	defer localFile.Close()

	if err := copyWithProgressAndStats(localFile, remoteFile, int(info.Size), showProgress); err != nil {
		fmt.Fprintln(os.Stderr, "error pulling file:", err)
		return 1
	}
	return 0
}

func push(showProgress bool, localPath, remotePath string, device adb.DeviceDescriptor) int {
	if remotePath == "" {
		fmt.Fprintln(os.Stderr, "error: must specify remote file")
		kingpin.Usage()
		return 1
	}

	var (
		localFile io.ReadCloser
		size      int
		perms     os.FileMode
		mtime     time.Time
	)
	if localPath == "" || localPath == StdIoFilename {
		localFile = os.Stdin
		// 0 size will hide the progress bar.
		perms = os.FileMode(0660)
		mtime = adb.MtimeOfClose
	} else {
		var err error
		localFile, err = os.Open(localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening local file %s: %s\n", localPath, err)
			return 1
		}
		info, err := os.Stat(localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading local file %s: %s\n", localPath, err)
			return 1
		}
		size = int(info.Size())
		perms = info.Mode().Perm()
		mtime = info.ModTime()
	}
	defer localFile.Close()

	client := client.Device(device)
	sc, writer, err := client.OpenFileWriter(remotePath, perms, mtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening remote file %s: %s\n", remotePath, err)
		return 1
	}
	defer writer.CopyDone()
	defer sc.Close()

	if err := copyWithProgressAndStats(writer, localFile, size, showProgress); err != nil {
		fmt.Fprintln(os.Stderr, "error pushing file:", err)
		return 1
	}
	return 0
}

// copyWithProgressAndStats copies src to dst.
// If showProgress is true and size is positive, a progress bar is shown.
// After copying, final stats about the transfer speed and size are shown.
// Progress and stats are printed to stderr.
func copyWithProgressAndStats(dst io.Writer, src io.Reader, size int, showProgress bool) error {
	var progress *pb.ProgressBar
	if showProgress && size > 0 {
		progress = pb.New(size)
		// Write to stderr in case dst is stdout.
		progress.Output = os.Stderr
		progress.ShowSpeed = true
		progress.ShowPercent = true
		progress.ShowTimeLeft = true
		progress.SetUnits(pb.U_BYTES)
		progress.Start()
		dst = io.MultiWriter(dst, progress)
	}

	startTime := time.Now()
	copied, err := io.Copy(dst, src)

	if progress != nil {
		progress.Finish()
	}

	if pathErr, ok := err.(*os.PathError); ok {
		if errno, ok := pathErr.Err.(syscall.Errno); ok && errno == syscall.EPIPE {
			// Pipe closed. Handle this like an EOF.
			err = nil
		}
	}
	if err != nil {
		return err
	}

	duration := time.Now().Sub(startTime)
	rate := int64(float64(copied) / duration.Seconds())
	fmt.Fprintf(os.Stderr, "%d B/s (%d bytes in %s)\n", rate, copied, duration)

	return nil
}

func push2(descriptor adb.DeviceDescriptor, localPath, remotePath string) int {
	device := client.Device(descriptor)
	err := device.PushDir(localPath, remotePath, true, func(totalFiles, sentFiles uint64, current string, percent, speed float64, err error) {
		if err != nil {
			fmt.Printf("[%d/%d] pushing %s, %.2f%%, err:%s\n", sentFiles, totalFiles, current, percent, err.Error())
		} else {
			fmt.Printf("[%d/%d] pushing %s, %.2f%%, %.02f MB/s\n", sentFiles, totalFiles, current, percent, speed)
		}
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "push failed:%v\n", err)
		return 1
	}
	return 0
}
