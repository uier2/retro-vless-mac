package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Session struct {
	PID                        int
	Core, Config, Device, Name string
	Verified                   bool
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func storeNodes(dir string, ns []Node) error {
	b, e := json.Marshal(ns)
	if e != nil {
		return e
	}
	tmp := filepath.Join(dir, "servers.new")
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(dir, "servers.json"))
}
func readNodes(dir string) ([]Node, error) {
	b, e := os.ReadFile(filepath.Join(dir, "servers.json"))
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var ns []Node
	e = json.Unmarshal(b, &ns)
	return ns, e
}
func listNodes(dir string) (string, error) {
	ns, e := readNodes(dir)
	if e != nil {
		return "", e
	}
	lines := []string{}
	for i, n := range ns {
		name := strings.Map(func(r rune) rune {
			if r < ' ' || r == 127 {
				return ' '
			}
			return r
		}, n.Name)
		lines = append(lines, fmt.Sprintf("%d\t%s", i, name))
	}
	return strings.Join(lines, "\n"), nil
}
func publicIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}
func fetchSubscription(link string) ([]Node, error) {
	kind, e := classify(link)
	if e != nil || kind != "subscription" {
		return nil, fmt.Errorf("нужна https-ссылка подписки")
	}
	tr := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		h, p, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, h)
		if e != nil {
			return nil, e
		}
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				return nil, fmt.Errorf("локальный адрес запрещён")
			}
		}
		for _, ip := range ips {
			c, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), p))
			if e == nil {
				return c, nil
			}
		}
		return nil, fmt.Errorf("нет связи с подпиской")
	}}
	defer tr.CloseIdleConnections()
	cl := &http.Client{Transport: tr, Timeout: 20 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 4 || r.URL.Scheme != "https" || r.URL.User != nil {
			return fmt.Errorf("небезопасное перенаправление")
		}
		return nil
	}}
	req, e := http.NewRequest("GET", link, nil)
	if e != nil {
		return nil, fmt.Errorf("неверная ссылка")
	}
	req.Header.Set("User-Agent", "sing-box/1.3.6 Retro VLESS")
	resp, e := cl.Do(req)
	if e != nil {
		return nil, fmt.Errorf("не удалось скачать подписку: проверьте интернет и срок действия ссылки")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("сервер подписки вернул HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if e != nil || len(b) > 4*1024*1024 {
		return nil, fmt.Errorf("слишком большая или повреждённая подписка")
	}
	return parseSubscription(b)
}
func session(dir string) (Session, error) {
	var s Session
	b, e := os.ReadFile(filepath.Join(dir, "session.json"))
	if e != nil {
		return s, e
	}
	e = json.Unmarshal(b, &s)
	return s, e
}
func running(s Session) bool {
	if s.PID < 2 || s.Core == "" || s.Config == "" {
		return false
	}
	b, e := exec.Command("/bin/ps", "-ww", "-p", strconv.Itoa(s.PID), "-o", "command=").Output()
	if e != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == s.Core+" run -c "+s.Config
}
func saveSession(dir string, s Session) error {
	b, _ := json.Marshal(s)
	return os.WriteFile(filepath.Join(dir, "session.json"), b, 0600)
}
func admin(command string) (string, error) {
	out, e := exec.Command("/usr/bin/osascript", "-e", "do shell script "+strconv.Quote(command)+" with administrator privileges").Output()
	if e != nil {
		return "", fmt.Errorf("действие отменено или macOS не разрешила запуск; нужен пароль администратора")
	}
	return strings.TrimSpace(string(out)), nil
}
func stopVPN(dir string) error {
	s, e := session(dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return fmt.Errorf("не удалось прочитать состояние подключения")
	}
	if running(s) {
		if _, e = admin("/bin/kill -TERM " + strconv.Itoa(s.PID)); e != nil {
			return e
		}
		for i := 0; i < 50; i++ {
			if !running(s) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if running(s) {
			return fmt.Errorf("клиент ещё завершает работу; попробуйте отключить ещё раз")
		}
	}
	os.Remove(filepath.Join(dir, "session.json"))
	os.Remove(filepath.Join(dir, "active.json"))
	return nil
}
func connectVPN(dir, core string, index int) error {
	if s, e := session(dir); e == nil && running(s) {
		return fmt.Errorf("сначала отключите текущее подключение")
	}
	ns, e := readNodes(dir)
	if e != nil || index < 0 || index >= len(ns) {
		return fmt.Errorf("сначала добавьте ссылку и выберите сервер")
	}
	u, _ := url.Parse(ns[index].Link)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
	if e != nil {
		return fmt.Errorf("не найден адрес сервера")
	}
	endpoint := ""
	for _, ip := range ips {
		if ip.IP.To4() != nil && publicIP(ip.IP) {
			endpoint = ip.IP.String()
			break
		}
	}
	if endpoint == "" {
		return fmt.Errorf("нужен сервер с публичным IPv4-адресом")
	}
	device := ""
	for i := 20; i < 100; i++ {
		d := fmt.Sprintf("utun%d", i)
		if _, e := net.InterfaceByName(d); e != nil {
			device = d
			break
		}
	}
	if device == "" {
		return fmt.Errorf("нет свободного интерфейса туннеля")
	}
	b, e := singConfig(ns[index], endpoint, device)
	if e != nil {
		return e
	}
	config := filepath.Join(dir, "active.json")
	if e = os.WriteFile(config, b, 0600); e != nil {
		return e
	}
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer checkCancel()
	if e = exec.CommandContext(checkCtx, core, "check", "-c", config).Run(); e != nil {
		return fmt.Errorf("клиент не принял конфигурацию или заблокирован macOS")
	}
	out, e := admin(shellQuote(core) + " run -c " + shellQuote(config) + " </dev/null >/dev/null 2>&1 & /bin/echo $!")
	if e != nil {
		return e
	}
	pid, e := strconv.Atoi(strings.TrimSpace(out))
	if e != nil || pid < 2 {
		return fmt.Errorf("клиент не вернул номер процесса")
	}
	s := Session{PID: pid, Core: core, Config: config, Device: device, Name: ns[index].Name}
	if e = saveSession(dir, s); e != nil {
		admin("/bin/kill -TERM " + strconv.Itoa(pid))
		return fmt.Errorf("не удалось сохранить состояние; подключение остановлено")
	}
	ready := false
	for i := 0; i < 50; i++ {
		if !running(s) {
			break
		}
		if _, e := net.InterfaceByName(device); e == nil {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		cleanupErr := stopVPN(dir)
		if cleanupErr != nil {
			return fmt.Errorf("туннель не запустился; нажмите отключить для завершения процесса")
		}
		return fmt.Errorf("туннель не запустился: другая VPN или несовместимая конфигурация")
	}
	route, e := exec.Command("/sbin/route", "-n", "get", "1.1.1.1").Output()
	if e != nil || !strings.Contains(string(route), "interface: "+device) {
		return fmt.Errorf("маршрут не подтверждён; нажмите отключить")
	}
	cl := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 12 * time.Second}
	defer cl.CloseIdleConnections()
	r, e := cl.Get("https://example.com")
	if e != nil {
		return fmt.Errorf("туннель запущен, но интернет не подтверждён; попробуйте отключить и выбрать другой сервер")
	}
	r.Body.Close()
	s.Verified = true
	return saveSession(dir, s)
}
func runCLI(dir, core string, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("нужна команда")
	}
	switch args[0] {
	case "status":
		s, e := session(dir)
		if e != nil || !running(s) {
			return "отключено", nil
		}
		if !s.Verified {
			return "туннель запущен, связь не подтверждена — отключите и попробуйте другой сервер", nil
		}
		if _, e = net.InterfaceByName(s.Device); e != nil {
			return "процесс работает, но туннель пропал — нажмите отключить", nil
		}
		return "подключено: " + s.Name, nil
	case "list":
		return listNodes(dir)
	case "import":
		if s, e := session(dir); e == nil && running(s) {
			return "", fmt.Errorf("сначала отключите VPN")
		}
		if len(args) != 2 {
			return "", fmt.Errorf("вставьте ссылку")
		}
		link := strings.TrimSpace(args[1])
		k, e := classify(link)
		if e != nil {
			return "", e
		}
		var ns []Node
		if k == "vless" {
			n, e := parseNode(link)
			if e != nil {
				return "", e
			}
			ns = []Node{n}
		} else {
			ns, e = fetchSubscription(link)
			if e != nil {
				return "", e
			}
		}
		if e = storeNodes(dir, ns); e != nil {
			return "", e
		}
		return fmt.Sprintf("сохранено серверов: %d. неподдерживаемые записи подписки пропущены", len(ns)), nil
	case "connect":
		if len(args) != 2 {
			return "", fmt.Errorf("выберите сервер")
		}
		i, e := strconv.Atoi(args[1])
		if e != nil {
			return "", fmt.Errorf("неверный сервер")
		}
		if e = connectVPN(dir, core, i); e != nil {
			return "", e
		}
		return "подключено. Chrome и Telegram используют общекомпьютерный туннель", nil
	case "disconnect":
		if e := stopVPN(dir); e != nil {
			return "", e
		}
		return "отключено", nil
	case "--version":
		return "Retro VLESS 1.0 / sing-box 1.3.6 / Intel macOS 10.13 target", nil
	}
	return "", fmt.Errorf("неизвестная команда")
}
func main() {
	home, e := os.UserHomeDir()
	if e != nil {
		os.Exit(1)
	}
	dir := filepath.Join(home, "Library/Application Support/Retro VLESS")
	if os.MkdirAll(dir, 0700) != nil {
		os.Exit(1)
	}
	os.Chmod(dir, 0700)
	lock, e := os.OpenFile(filepath.Join(dir, "operation.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		os.Exit(1)
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		fmt.Fprintln(os.Stderr, "другая операция ещё выполняется")
		os.Exit(1)
	}
	exe, _ := os.Executable()
	out, e := runCLI(dir, filepath.Join(filepath.Dir(exe), "sing-box"), os.Args[1:])
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println(out)
}
