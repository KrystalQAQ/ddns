package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	ipifyURL      = "https://api.ip.sb/ip"
	currentIPFile = "/data/current_ip.txt"
)

type Config struct {
	APIToken     string
	Domain       string
	Subdomain    string
	CheckInterval time.Duration
}

type CloudflareZoneResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type CloudflareDNSResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Content string `json:"content"`
		Type    string `json:"type"`
	} `json:"result"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type CloudflareUpdateResponse struct {
	Success bool `json:"success"`
	Result  struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Content string `json:"content"`
	} `json:"result"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

var config Config
var fullDomain string

func getTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func getPublicIP() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ipifyURL)
	if err != nil {
		return "", fmt.Errorf("获取IP失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("获取IP失败，状态码: %d", resp.StatusCode)
	}

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取IP响应失败: %v", err)
	}

	return strings.TrimSpace(string(ip)), nil
}

func apiRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}

func getZoneID() (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", config.Domain)
	resp, err := apiRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result CloudflareZoneResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return "", fmt.Errorf("API错误: %s", result.Errors[0].Message)
		}
		return "", fmt.Errorf("获取Zone ID失败")
	}

	if len(result.Result) == 0 {
		return "", fmt.Errorf("未找到域名: %s", config.Domain)
	}

	return result.Result[0].ID, nil
}

func getDNSRecordID(zoneID string) (string, string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=A&name=%s", zoneID, fullDomain)
	resp, err := apiRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var result CloudflareDNSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", err
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return "", "", fmt.Errorf("API错误: %s", result.Errors[0].Message)
		}
		return "", "", fmt.Errorf("获取DNS记录失败")
	}

	if len(result.Result) == 0 {
		return "", "", fmt.Errorf("未找到DNS记录: %s", fullDomain)
	}

	return result.Result[0].ID, result.Result[0].Content, nil
}

func updateDNSRecord(zoneID, recordID, newIP string) error {
	payload := map[string]interface{}{
		"type":    "A",
		"name":    fullDomain,
		"content": newIP,
		"ttl":     120, // 2分钟
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID)
	resp, err := apiRequest("PATCH", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result CloudflareUpdateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return err
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return fmt.Errorf("更新失败: %s", result.Errors[0].Message)
		}
		return fmt.Errorf("更新DNS记录失败")
	}

	return nil
}

func saveIPToFile(ip string) error {
	return os.WriteFile(currentIPFile, []byte(ip), 0644)
}

func readIPFromFile() (string, error) {
	data, err := os.ReadFile(currentIPFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func checkAndUpdateIP(zoneID string) error {
	fmt.Printf("[%s] 🔍 正在获取当前公网IP...\n", getTimestamp())

	currentIP, err := getPublicIP()
	if err != nil {
		return err
	}

	fmt.Printf("[%s] 📍 当前公网IP: %s\n", getTimestamp(), currentIP)

	lastIP, _ := readIPFromFile()

	if currentIP == lastIP {
		fmt.Printf("[%s] ✅ IP未变化，无需更新\n\n", getTimestamp())
		return nil
	}

	fmt.Printf("[%s] 🔄 检测到IP变化: %s -> %s\n", getTimestamp(), lastIPOrUnknown(lastIP), currentIP)
	fmt.Printf("[%s] 🔄 正在更新Cloudflare DNS记录...\n", getTimestamp())

	recordID, oldIP, err := getDNSRecordID(zoneID)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] 📝 DNS记录ID: %s\n", getTimestamp(), recordID)
	fmt.Printf("[%s] 📝 原DNS IP: %s\n", getTimestamp(), oldIP)

	if err := updateDNSRecord(zoneID, recordID, currentIP); err != nil {
		return err
	}

	if err := saveIPToFile(currentIP); err != nil {
		fmt.Printf("[%s] ⚠️  保存IP文件失败: %v\n", getTimestamp(), err)
	}

	fmt.Printf("[%s] ✅ DNS记录更新成功!\n", getTimestamp())
	fmt.Printf("[%s] ✅ %s -> %s\n\n", getTimestamp(), fullDomain, currentIP)

	return nil
}

func lastIPOrUnknown(lastIP string) string {
	if lastIP == "" {
		return "(首次运行)"
	}
	return lastIP
}

func run() error {
	// 获取Zone ID
	fmt.Printf("[%s] 🔍 正在获取Zone ID...\n", getTimestamp())
	zoneID, err := getZoneID()
	if err != nil {
		return err
	}
	fmt.Printf("[%s] ✅ Zone ID: %s\n\n", getTimestamp(), zoneID)

	// 立即执行一次
	if err := checkAndUpdateIP(zoneID); err != nil {
		return err
	}

	// 设置定时器
	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()

	fmt.Printf("[%s] ⏰ 等待 %.0f 分钟后进行下次检查...\n\n", getTimestamp(), config.CheckInterval.Minutes())

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			if err := checkAndUpdateIP(zoneID); err != nil {
				fmt.Printf("[%s] ❌ 错误: %v\n\n", getTimestamp(), err)
			}
			fmt.Printf("[%s] ⏰ 等待 %.0f 分钟后进行下次检查...\n\n", getTimestamp(), config.CheckInterval.Minutes())
		case <-sigChan:
			fmt.Printf("\n[%s] 👋 收到中断信号，程序退出\n", getTimestamp())
			return nil
		}
	}
}

func main() {
	fmt.Print("\n🚀 Cloudflare DDNS 启动\n")
	fmt.Println("========================================")

	// 读取环境变量
	config.APIToken = os.Getenv("CLOUDFLARE_API_TOKEN")
	config.Domain = os.Getenv("DOMAIN")
	config.Subdomain = os.Getenv("SUBDOMAIN")
	if config.Subdomain == "" {
		config.Subdomain = "ddns"
	}

	interval := os.Getenv("CHECK_INTERVAL")
	if interval == "" {
		interval = "5"
	}
	var err error
	config.CheckInterval, err = time.ParseDuration(interval + "m")
	if err != nil {
		fmt.Printf("⚠️  检查间隔格式错误，使用默认值5分钟\n")
		config.CheckInterval = 5 * time.Minute
	}

	// 验证配置
	if config.APIToken == "" || config.APIToken == "your_api_token_here" {
		fmt.Println("❌ 错误: 请设置环境变量 CLOUDFLARE_API_TOKEN")
		os.Exit(1)
	}

	if config.Domain == "" || config.Domain == "example.com" {
		fmt.Println("❌ 错误: 请设置环境变量 DOMAIN")
		os.Exit(1)
	}

	// 构建完整域名
	if config.Subdomain == "@" {
		fullDomain = config.Domain
	} else {
		fullDomain = fmt.Sprintf("%s.%s", config.Subdomain, config.Domain)
	}

	// 打印配置信息
	fmt.Printf("📋 配置信息:\n")
	fmt.Printf("   域名: %s\n", fullDomain)
	fmt.Printf("   检查间隔: %.0f 分钟\n\n", config.CheckInterval.Minutes())

	// 运行主程序
	if err := run(); err != nil {
		fmt.Printf("[%s] ❌ 程序异常退出: %v\n", getTimestamp(), err)
		os.Exit(1)
	}
}
