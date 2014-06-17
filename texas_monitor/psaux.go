package main

import (
	"fmt"
	"strings"

	"bytes"
	"os/exec"
	"strconv"

	"io/ioutil"
	"time"
)

func getCPUSample() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					fmt.Println("Error: ", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}

func monitorCpu() {
	idle0, total0 := getCPUSample()
	time.Sleep(3 * time.Second)
	idle1, total1 := getCPUSample()

	idleTicks := float64(idle1 - idle0)
	totalTicks := float64(total1 - total0)
	cpuUsage := 100 * (totalTicks - idleTicks) / totalTicks

	fmt.Printf("CPU usage is %f%% [busy: %f, total: %f]\n", cpuUsage, totalTicks-idleTicks, totalTicks)
}

type Process struct {
	name string
	pid  int
	cpu  float64
}

func GetCpuStat(grep []string) (stat []*Process, ret int) {

	var command string
	if len(grep) != 0 {

		command = "ps aux | grep -E '"
		for i, name := range grep {
			if i == 0 {
				command += name
			} else {
				command += "|" + name
			}
		}
		command += "'| grep -v 'grep'\n"

	} else {
		command = "ps aux"
	}

	/*fmt.Println(command)*/

	cmd := exec.Command("/bin/sh", "-c", command)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		/*fmt.Println(err)*/
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return nil, -1
	}
	processes := make([]*Process, 0)
	for {
		line, err := out.ReadString('\n')
		if err != nil {
			break
		}
		tokens := strings.Split(line, " ")
		ft := make([]string, 0)
		for _, t := range tokens {
			if t != "" && t != "\t" {
				ft = append(ft, t)
			}
		}
		/*log.Println(len(ft), ft)*/
		name := ft[10]

		pid, err := strconv.Atoi(ft[1])
		if err != nil {
			continue
		}
		cpu, err := strconv.ParseFloat(ft[2], 64)
		if err != nil {
			fmt.Println(err)
			continue
		}
		processes = append(processes, &Process{name, pid, cpu})
	}
	/*for _, p := range(processes) {*/
	/*fmt.Println(p.name, p.pid, " takes ", p.cpu, " % of the CPU")*/
	/*}*/

	return processes, 0
}
