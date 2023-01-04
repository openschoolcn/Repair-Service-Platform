package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	api         = "https://hq.xcc.edu.cn/rsp/site/DataCxlist"
	proxyServer = "161.189.51.250:20029"
	pageSize    = 100 // 每页的数据
)

var (
	rowCount = 0
	proxyURL = &url.URL{}
)

func main() {

	now := time.Now()

	// 初始化代理
	InitProxyURL()

	// 获取第一页数据,获取rowCount
	InitFirstPage()

	// 向上取整
	pageIndex := int(math.Ceil(float64(rowCount) / pageSize))
	// 从第二页开始
	wg := &sync.WaitGroup{}
	for i := 2; i < pageIndex; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := run(strconv.Itoa(i)); err != nil {
				return
			}
		}(i)
	}
	wg.Wait()

	fmt.Printf("==========爬取完成,总耗时:%v==========\n", time.Since(now))
}

// InitFirstPage 获取第一页数据,获取rowCount
func InitFirstPage() {
	first := "1"
	b, err := request(first)
	if err != nil {
		panic(err)
	}

	result, err := readHTMLValue(b, true)
	if err != nil {
		panic(err)
	}

	if err := writeFile(first, strconv.Itoa(pageSize), result); err != nil {
		panic(err)
	}
}

// InitProxyURL 初始化代理
func InitProxyURL() {
	val, err := url.Parse("http://" + proxyServer)
	if err != nil {
		panic(err)
	}

	proxyURL = val
}

// request 发起请求
func request(pageIndex string) ([]byte, error) {
	postData := url.Values{}
	postData.Add("RowCount", strconv.Itoa(rowCount))
	postData.Add("PageIndex", pageIndex)
	postData.Add("PageSize", strconv.Itoa(pageSize))
	postData.Add("Bstate", "all")
	postData.Add("t", "all")
	postData.Add("tw", "all")
	postData.Add("phone", "all")

	request, err := http.NewRequest(http.MethodPost, api, strings.NewReader(postData.Encode()))
	if err != nil {
		return nil, err
	}

	request.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	client := &http.Client{Timeout: 5 * time.Minute, Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	defer client.CloseIdleConnections()

	return io.ReadAll(resp.Body)
}

// readHTMLValue 读取网页内容
func readHTMLValue(b []byte, first bool) ([]byte, error) {
	dom, err := goquery.NewDocumentFromReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	if first {
		val, exists := dom.Find("input").Attr("value")
		if exists {
			rowCount, _ = strconv.Atoi(val)
		}
	}

	dataList := make([]map[string]any, 0, pageSize)

	dom.Find("ul").Each(func(i int, _selection *goquery.Selection) {
		data := make(map[string]any, pageSize)
		_selection.Find("li").Each(func(i int, selection *goquery.Selection) {
			selection.Find("nobr").Each(func(i int, s *goquery.Selection) {
				key, _ := selection.Attr("class")
				key = strings.Replace(key, " ", "_", -1)

				content := s.Find("em").Text()
				if content != "" {
					data[key] = strings.Trim(content, " ")
					return
				}

				data[key] = strings.Trim(s.Text(), " ")
			})
		})
		dataList = append(dataList, data)
	})

	b, err = json.Marshal(&dataList)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// writeFile 写入文件
func writeFile(page, pageSize string, b []byte) error {
	fileName := fmt.Sprintf("./json/%s_%s.json", page, pageSize)
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write(b) //写入字节切片数据

	return nil
}

// run 启动程序
func run(pageIndex string) error {
	b, err := request(pageIndex)
	if err != nil {
		log.Printf("request err:%v", err)
		return err
	}

	result, err := readHTMLValue(b, true)
	if err != nil {
		log.Printf("readHTMLValue err:%v", err)
		return err
	}

	if err := writeFile(pageIndex, strconv.Itoa(pageSize), result); err != nil {
		log.Printf("writeFile err:%v", err)
		return err
	}

	return nil
}
