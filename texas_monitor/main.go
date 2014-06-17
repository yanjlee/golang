package main

import (
	"bitbucket.org/bertimus9/systemstat"
	"code.google.com/p/gcfg"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type monitor struct {
	interval int32

	memUsedLimit  uint64
	loadAvgLimit  float64
	diskUsedLimit uint64
	cpuUsedLimit  float64

	exitChan  chan int
	mail      []string
	process   []string
	waitGroup sync.WaitGroup
}

func (m *monitor) Wrap(cb func()) {
	m.waitGroup.Add(1)
	go func() {
		cb()
		m.waitGroup.Done()
	}()
}

func (m *monitor) main() {
	m.Wrap(func() { memMonitor(m) })
	m.Wrap(func() { loadMonitor(m) })
	m.Wrap(func() { diskMonitor(m) })
	m.Wrap(func() { cpuMonitor(m) })
}

func (m *monitor) exit() {
	close(m.exitChan)
	m.waitGroup.Wait()
	fmt.Println("texas_monitor exit")
}

func (m *monitor) sendMail(title string, body string) {

	for _, name := range m.mail {

		log.Println("send mail to: ", name)
                ip := getIP()
                hostname, err := os.Hostname()
                if err != nil {
                   log.Println("get hostname failed")
                   hostname = ""
                }

                SendMail(title, "Host:" + hostname + ", IP:"+ ip + "," + body, name)
	}
}

func memMonitor(m *monitor) {
	/*mem*/
	ticker := time.NewTicker(time.Minute * time.Duration(m.interval))

	for {
		select {
		case <-m.exitChan:
			goto finish

		case <-ticker.C:
			mem := systemstat.GetMemSample()
			memUsed := mem.MemUsed - mem.Buffers - mem.Cached
			memFree := mem.MemFree + mem.Buffers + mem.Cached
			memUsedPercentage := memUsed * 100 / mem.MemTotal

			log.Println(fmt.Sprintf("RAM: percentage:%d%% used:%d MB, free:%d MB, total:%d MB", memUsedPercentage, memUsed/1024, memFree/1024, mem.MemTotal/1024))
			if memUsedPercentage > m.memUsedLimit {
				/*mail*/
				a := time.Now()

				title := "内存占用" + fmt.Sprintf("%d%%", memUsedPercentage)
				body := a.UTC().String() + "内存使用:" + fmt.Sprintf("%d%%", memUsedPercentage)
				log.Println("mem warnning")
				m.sendMail(title, body)
			}
		}
	}

finish:
}

func loadMonitor(m *monitor) {
	/*load avg*/
	ticker := time.NewTicker(time.Minute * time.Duration(m.interval))

	for {
		select {
		case <-m.exitChan:
			goto finish

		case <-ticker.C:
			loadSample := systemstat.GetLoadAvgSample()
			log.Println(fmt.Sprintf("Load avg: (1)%2.2f  (5)%2.2f  (15)%2.2f", loadSample.One, loadSample.Five, loadSample.Fifteen))
			if loadSample.Fifteen > m.loadAvgLimit {
				log.Println("load Avg warnning")
				a := time.Now()
                                title := "load Avg过大:" + fmt.Sprintf("%2f", loadSample.Fifteen)
				body := a.UTC().String() + "15分钟loadAvg=" + fmt.Sprintf("%2f", loadSample.Fifteen)
				m.sendMail(title, body)
			}
		}
	}

finish:
}

func diskMonitor(m *monitor) {
	/*disk space usage*/
	ticker := time.NewTicker(time.Minute * time.Duration(m.interval))

	for {
		select {
		case <-m.exitChan:
			goto finish

		case <-ticker.C:
			diskStatus, err := GetDiskStatus("/")
			if err != nil {
				log.Println("get disk status failed!")
			}

			diskPercentage := diskStatus.Used * 100 / diskStatus.All

			log.Println(fmt.Sprintf("Disk:percentage:%d%%, all: %dGB, used: %dGB, free: %dGB",
				diskPercentage, diskStatus.All/1024/1024/1024,
				diskStatus.Used/1024/1024/1024, diskStatus.Free/1024/1024/1024))

			if diskPercentage > m.diskUsedLimit {
				log.Println("disk warnning")
				a := time.Now()
				title := "磁盘使用率过高" + fmt.Sprintf("%d", diskPercentage)
				body := a.UTC().String() + "使用率=" + fmt.Sprintf("%d", diskPercentage)
				m.sendMail(title, body)
			}
		}
	}

finish:
}

func cpuMonitor(m *monitor) {
	/*pthread cpu status*/
	ticker := time.NewTicker(time.Minute * time.Duration(m.interval))

	for {
		select {
		case <-m.exitChan:
			goto finish

		case <-ticker.C:
			stat, ret := GetCpuStat(m.process)
			if ret == -1 {
				/*没有进程，是否报警*/

			} else {
				/*check stat*/
				for _, subStat := range stat {
					log.Println(fmt.Sprintf("task: cpu: %3.2f%% pid:%6d  %s", subStat.cpu, subStat.pid, subStat.name))
					if subStat.cpu > m.cpuUsedLimit {
						log.Println("cpu warnning")
						a := time.Now()
						title := "cpu使用率过高"
						body := a.UTC().String() + fmt.Sprintf(" process name:%s, cpu:%3.2f%%", subStat.name, subStat.cpu)
						m.sendMail(title, body)
					}

				}

			}
		}
	}

finish:
}

type Config struct {
	Monitor struct {
		Interval      int32
		MemUsedLimit  uint64
		LoadAvgLimit  float64
		DiskUsedLimit uint64
		CpuUsedLimit  float64
	}

	ProcessName struct {
		Name []string
	}

	Mail struct {
		Addr []string
	}
}

func generalConfigFile() {
	b := []byte(
`[Monitor]
Interval=30      #采样间隔 单位:分钟
MemUsedLimit=80  #内存使用百分比上限
LoadAvgLimit=4   #load avg 15min 上限
DiskUsedLimit=80 #磁盘使用百分比上限
CpuUsedLimit=90  #监控进程cpu占用上限百分比

[ProcessName]
Name=texas_server #监控cpu服务名称关键字(可多个, Name为空时, 监控所有进程)
Name=mysqld

[Mail]
Addr=13825251001@139.com  #告警通知邮件(可多个)
Addr=13927412726@139.com
Addr=13760119951@139.com
`)
	err := ioutil.WriteFile("./texas_monitor.ini", b, 0644)
	if err != nil {
		log.Fatalf("Failed to general texas_monitor.ini: %s", err)
		return
	}
	fmt.Println("general ./texas_monitor.ini successful")
}

var (
	// basic options
	/*showVersion = flag.Bool("version", false, "print version string")*/
	verbose          = flag.Bool("verbose", false, "enable verbose logging")
	outPutConfigFile = flag.Bool("o", false, "generate the texas_monitor.ini file")
)

func main() {

	var cfg Config

	flag.Parse()

	if *outPutConfigFile {
		generalConfigFile()
		return
	}

	err := gcfg.ReadFileInto(&cfg, "./texas_monitor.ini")
	if err != nil {
		log.Fatalf("Failed to parse texas_monitor.ini: %s", err)
	}

	fmt.Println("interval:", cfg.Monitor.Interval)
	fmt.Println("memUsedLimit:", cfg.Monitor.MemUsedLimit)
	fmt.Println("loadAvgLimit:", cfg.Monitor.LoadAvgLimit)
	fmt.Println("diskUsedLimit:", cfg.Monitor.DiskUsedLimit)
	fmt.Println("cpuUsedLimit:", cfg.Monitor.CpuUsedLimit)
	fmt.Println("Process:", cfg.ProcessName.Name)
	fmt.Println("Mail:", cfg.Mail.Addr)

	f, err := os.OpenFile("monitor_log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	defer f.Close()

	log.SetOutput(f)

	/*log.Println("This is a test log entry")*/
	exitChan := make(chan int)
	signalChan := make(chan os.Signal, 1)
	go func() {
		<-signalChan
		exitChan <- 1
	}()
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	monitor := &monitor{
		interval: 30,
		exitChan: make(chan int),
	}

	monitor.interval = cfg.Monitor.Interval
	monitor.memUsedLimit = cfg.Monitor.MemUsedLimit
	monitor.loadAvgLimit = cfg.Monitor.LoadAvgLimit
	monitor.diskUsedLimit = cfg.Monitor.DiskUsedLimit
	monitor.cpuUsedLimit = cfg.Monitor.CpuUsedLimit
	monitor.mail = cfg.Mail.Addr
	monitor.process = cfg.ProcessName.Name

	monitor.main()

	<-exitChan

	monitor.exit()
}

func filedIP(ipnet *net.IPNet) bool {
	if ip4 := ipnet.IP.To4(); ip4 != nil {
		if ip4[0] == 127 || ip4[0] == 172 || ip4[0] == 10 {
			return true
		}
	}

	return false
}

func getIP() (a string) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Println("getIP: " + err.Error())
		return
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !filedIP(ipnet) {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return
}
