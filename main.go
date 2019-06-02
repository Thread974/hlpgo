package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HttpMethod int

const (
	HttpGet HttpMethod = 0
	HttpPost HttpMethod = 1
	HttpMethodCount HttpMethod = 2
)

const NANOINSEC = 1000000000

func (m HttpMethod) String() string {
	if m == HttpGet {
		return "GET"
	}
	if m == HttpPost {
		return "POST"
	}
	return "Invalid"
}

func HttpMethodFromString(s string) HttpMethod {
	if s == "GET" {
		return HttpGet
	}
	return HttpPost
}

type Logentry struct {
	ip          string
	user        string
	timestamp   string
	timezone    string
	method      HttpMethod
	path        string
	httpversion string
	code        string
	size        string
}

func (l Logentry) String() string {
	return fmt.Sprintf("%s - %s [%s %s] \"%s %s %s\" %s %s",
					l.ip, l.user, l.timestamp, l.timezone,
					l.method, l.path, l.httpversion, l.code, l.size)
}

func LogentryParse(l *Logentry, s string) error {

	if len(s) > 100000 {
		return errors.New("Logentry too big")
	}

	var words = strings.Split(s, " ")

	if len(words) != 10 {
		return errors.New("Cannot parse logentry, too many keywords")
	}

	l.ip          = words[0]
	l.user        = words[2]
	l.timestamp   = words[3][1:]
	l.timezone    = words[4][:len(words[4])-1]
	l.method      = HttpMethodFromString(words[5][1:])
	l.path        = words[6]
	l.httpversion = words[7][:len(words[7])-1]
	l.code        = words[8]
	l.size        = words[9]

	return nil
}

func LogentryGenerate(l *Logentry) {
	ips := []string {
		"127.0.0.1",
		"192.168.1.1" }
	users := []string {
		"fredo",
		"emilie",
		"julien" }
	methods := []string {
		"GET",
		"POST",
		"CONNECT" }
	paths := []string {
		"/main/a",
		"/main/b",
		"/main/c",
		"/main/d",
		"/main/e",
		"/main/f",
		"/main/g",
		"/main/h",
		"/main/i",
		"/toto/a",
		"/toto/a" }

	l.ip          = ips[rand.Intn(len(ips))]
	l.user        = users[rand.Intn(len(users))]
	l.timestamp   = time.Now().Format("01/Jun/2018:15:54:00")
	l.timezone    = "+0000"
	l.method      = HttpMethodFromString(methods[rand.Intn(int(HttpMethodCount))])
	l.path        = paths[rand.Intn(len(paths))]
	l.httpversion = "HTTP/1.0"
	l.code        = fmt.Sprintf("%d", 200)
	l.size        = fmt.Sprintf("%d", rand.Intn(500));
}

func TestParser() error {
	entries := []string {
		"127.0.0.1 - james [09/May/2018:16:00:39 +0000] \"GET /report HTTP/1.0\" 200 123",
		"127.0.0.1 - jill [09/May/2018:16:00:41 +0000] \"GET /api/user HTTP/1.0\" 200 234",
		"127.0.0.1 - frank [09/May/2018:16:00:42 +0000] \"POST /api/user HTTP/1.0\" 200 34",
		"127.0.0.1 - mary [09/May/2018:16:00:42 +0000] \"POST /api/user HTTP/1.0\" 503 12" }

	for _, entry := range entries {
		var logentry Logentry
		var formatted string

		LogentryParse(&logentry, entry)
		formatted = logentry.String()

		if formatted != entry {
			fmt.Println("Failed:");
			fmt.Println("\tentry", entry)
			fmt.Println("\tparsed", logentry.String())
			fmt.Println("\toutput", formatted)
			return errors.New("Test failed");
		}
	}

	return nil
}

func gen(quit chan int, filename string, delay time.Duration) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for {
		select {
			case <-quit:
				return nil
			default:
		}

		var logentry Logentry
		LogentryGenerate(&logentry)
		file.WriteString(logentry.String());
		file.WriteString("\n");
		if (delay != 0) {
			time.Sleep(delay)
		}
	}

	return nil
}

type LineHandler interface {
	init()
	// Accessed from multiple goroutines
	process(line *string,  logentry *Logentry) error
	// Accessed from multiple goroutines
	display()
	fini()
}

func monitor(quit chan int, filename string, linehandlers []LineHandler) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	fmt.Println("Scanning from byte", offset);

	reader := bufio.NewReader(file)

	fmt.Println("Scanning first line");

	var buf string
	for {
		// The reader sometimes returns incomplete lines
		// Make sure to have full lines before parsing
		var line string
		for len(line) == 0 || (len(line) < 40000 && line[len(line)-1] != '\n') {
			select {
				case <-quit:
					return nil
				default:
			}
			line, err = reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return err
			}
			buf = buf + line
		}
		line = buf
		buf = ""


		if err == nil && len(line)>1 {
			var logentry Logentry
			err = LogentryParse(&logentry, line)
			if err == nil {
				for _, linehandler := range linehandlers {
					linehandler.process(&line, &logentry)
				}
			} else {
				fmt.Printf("Parsing error[%s]\n", line)
			}
		}
	}

	return nil
}

type PrintLineHandler struct { }

func (p *PrintLineHandler)init() {
	fmt.Println("PrintLineHandler ready")
}

func (p *PrintLineHandler)process(line *string, logentry *Logentry) error {
	fmt.Printf("\"%s\"\n", *line)
	return nil
}

func (p *PrintLineHandler)display() {
	// No synchronisation because the display method does nothing
}

func (p *PrintLineHandler)fini() {
	fmt.Println("PrintLineHandler fini")
}

type PerURLStatisticsLineHandler struct {
	nanosec time.Duration
	start time.Time
	sections map[string]int
	m sync.Mutex
}

func (p *PerURLStatisticsLineHandler)init() {
	p.start = time.Now()
	p.sections = make(map[string]int)
	fmt.Println("PerURLStatisticsLineHandler ready")
}

func (p *PerURLStatisticsLineHandler)process(line *string, logentry *Logentry) error {
	p.m.Lock()
	defer p.m.Unlock()

	var section []string = strings.Split(logentry.path, "/")
	if (logentry.path[0] == '/') {
		p.sections[section[1]] ++
	} else {
		p.sections[section[0]] ++
	}
	return nil
}

func (p *PerURLStatisticsLineHandler)display() {
	p.m.Lock()
	defer p.m.Unlock()

	if (time.Since(p.start) > p.nanosec) {
		fmt.Println("Statistics:")
		var total = 0
		for key, value := range p.sections {
			fmt.Println("\tSection", key, "Hits", value)
			total += value
		}
		fmt.Println("\tTotal requests:", total)
		p.start = time.Now()
		p.sections = make(map[string]int)
	}
}

func (p *PerURLStatisticsLineHandler)fini() {
	fmt.Println("PerURLStatisticsLineHandler fini")
}

type GlobalStatisticsLineHandler struct {
	nanosec time.Duration
	requests uint64
	bytessent uint64
	secs uint64
	threshold uint64
	start time.Time
	m sync.Mutex
	alert bool
}

func (p *GlobalStatisticsLineHandler)init() {
	p.requests = 0
	p.bytessent = 0
	p.secs = uint64(p.nanosec) / NANOINSEC
	p.start = time.Now()
	fmt.Println("GlobalStatisticsLineHandler ready")
}

func (p *GlobalStatisticsLineHandler)process(line *string, logentry *Logentry) error {
	p.m.Lock()
	defer p.m.Unlock()

	size, err := strconv.Atoi(strings.Trim(logentry.size, " \t\r\n"))
	if err != nil {
		fmt.Println("GlobalStatisticsLineHandler", logentry.size, "converted to", size)
		return errors.New("Failed to convert an int")
	}
	if size < 0 {
		return errors.New("Parsed a negative size")
	}
	p.bytessent += uint64(size)
	p.requests ++
	return nil
}

func (p *GlobalStatisticsLineHandler)display() {
	p.m.Lock()
	defer p.m.Unlock()

	if (time.Since(p.start) > p.nanosec) {
		fmt.Println("Global statistics:");
		fmt.Println("\tRequests:", p.requests, "Bytes sent:", p.bytessent)

		if (p.requests / p.secs > p.threshold && !p.alert) {
			fmt.Println("*** High traffic generated an alert at ", time.Now(), ", requests:", p.requests, "***")
			p.alert = true
		}

		if (p.requests / p.secs < p.threshold && p.alert) {
			fmt.Println("*** High traffic alert recovered at ", time.Now(), ", , requests:", p.requests, "***")
			p.alert = false
		}

		p.requests = 0
		p.bytessent = 0
		p.start = time.Now()
	}
}

func (p *GlobalStatisticsLineHandler)fini() {
	fmt.Println("GlobalStatisticsLineHandler fini")
}

func ui(quit chan int, linehandlers []LineHandler) error {
	for {
		select {
			case <-quit:
				return nil
			default:
		}

		time.Sleep(10*1000*1000);

		for _, linehandler := range linehandlers {
			linehandler.display()
		}
	}

	return nil
}

func TestAlert() error {

	entry := "127.0.0.1 - james [09/May/2018:16:00:39 +0000] \"GET /report HTTP/1.0\" 200 123"

	var logentry Logentry
	LogentryParse(&logentry, entry)

	// Alert if more than 20 requests per seconds
	var toto = GlobalStatisticsLineHandler{nanosec: 1*NANOINSEC, threshold: 20}
	toto.init()

	// Display 10 times in a second, no alert
	for i := 0; i < 10; i++ {
		toto.process(&entry, &logentry)
		time.Sleep(NANOINSEC/10)
	}
	toto.display()
	if toto.alert {
		return errors.New("Test failed, alert should not be there");
	}

	// Display 30 times in a second, no alert
	for i := 0; i < 30; i++ {
		toto.process(&entry, &logentry)
		time.Sleep(NANOINSEC/30)
	}
	toto.display()
	if !toto.alert {
		return errors.New("Test failed, alert state should be set");
	}

	// Display 10 times in a second, alert recovers
	for i := 0; i < 10; i++ {
		toto.process(&entry, &logentry)
		time.Sleep(NANOINSEC/10)
	}
	toto.display()
	if toto.alert {
		return errors.New("Test failed, alert should be recovered");
	}
	
	return nil
}

func main() {
	logfileP := flag.String("file", "/tmp/access.log", "Filename to monitor")
	genP := flag.Bool("generate", false, "Generate data on the fly")
	genDurationP := flag.Duration("generate-internal", 0, "Generate data internel")
	runtestP := flag.Bool("test", false, "Run test")
	printP := flag.Bool("print", false, "Print input data")
	statsP := flag.Bool("statistics", true, "Show section access")
	globsP := flag.Bool("global", true, "Show global statistics")
	statsdelayP := flag.Duration("stats-delay", 2 * NANOINSEC, "Show section access delay")
	globsdelayP := flag.Duration("globs-delay", 3 * NANOINSEC, "Show global statistics delay")
	thresholdP := flag.Uint64("threshold", 300000, "Global alarm threshold")
	rundelayP := flag.Duration("run-delay", 0, "Quit after this delay expired")
	helpP := flag.Bool("help", false, "Show help")
	flag.Parse()

	if (*helpP) {
		flag.PrintDefaults()
		return
	}

	if (*runtestP) {
		fmt.Println("Testing the logentry parser")
		if err := TestParser(); err != nil {
			log.Fatal(err)
		}

		fmt.Println("Testing the alerting logic")
		if err := TestAlert(); err != nil {
			log.Fatal(err)
		}

		fmt.Println("All tests passed")
		return
	}

	fmt.Println("Starting monitoring file", *logfileP)

	var quit = make(chan int)

	if (*genP) {
		go gen(quit, *logfileP, *genDurationP)
	}

	var linehandlers []LineHandler
	if (*printP) {
		linehandlers = append(linehandlers, &PrintLineHandler{})
	}

	if (*statsP) {
		linehandlers = append(linehandlers, &PerURLStatisticsLineHandler{nanosec: *statsdelayP})
	}

	if (*globsP) {
		linehandlers = append(linehandlers, &GlobalStatisticsLineHandler{nanosec: *globsdelayP, threshold: *thresholdP})
	}

	for _, linehandler := range linehandlers {
		linehandler.init()
	}

	go monitor(quit, *logfileP, linehandlers)

	go ui(quit, linehandlers)

	if (*rundelayP == 0) {
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')
	} else {
		time.Sleep(*rundelayP);
	}

	if (*genP) {
		quit <- 1 // gen
	}
	quit <- 2 // monitor
	quit <- 3 // ui

	for _, linehandler := range linehandlers {
		linehandler.fini()
	}
}
