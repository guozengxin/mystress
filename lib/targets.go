package stress

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

type Target struct {
	Method string
	URL    string
	Body   []byte
	File   string
	Header http.Header
}

type Targets []Target

func NewTargetsFrom(source io.Reader, body []byte, header http.Header) (Targets, error) {
	scanner := bufio.NewScanner(source)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line = strings.TrimSpace(line); line != "" && line[0:2] != "//" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return NewTargets(lines, body, header)
}

func NewTargets(lines []string, body []byte, header http.Header) (Targets, error) {
	var targets Targets
	for _, line := range lines {
		ps := strings.Split(line, " ")
		argc := len(ps)
		if argc >= 2 {
			newHeader := http.Header{}
			for k, vs := range header {
				newHeader[k] = make([]string, len(vs))
				copy(newHeader[k], vs)
			}
			i := 0
			method := ps[i]
			i++
			if strings.Contains(ps[i], "http") == false {
				for ; i < len(ps) && strings.Contains(ps[i], "http") == false; i++ {
					kv := strings.Split(ps[i], ":")
					if len(kv) != 2 {
						continue
					} else {
						newHeader.Set(kv[0], kv[1])
					}
				}
			}
			var url, postFile string
			if i < argc {
				url = ps[i]
			} else {
				url = ""
			}
			i++
			if i < argc {
				postFile = ps[i]
			} else {
				postFile = ""
			}
			if url != "" {
				targets = append(targets, Target{Method: method, URL: url, File: postFile, Body: body, Header: newHeader})
			}
		} else {
			return nil, fmt.Errorf("Invalid request format: `%s`", line)
		}
	}
	return targets, nil
}

func (t *Target) Request() (*http.Request, error) {
	var req *http.Request
	var err error
	if t.Method == "POST" && t.File != "" {
		if strings.Contains(t.File, "form") {
			buf := &bytes.Buffer{}
			w := multipart.NewWriter(buf)
			kv := strings.Split(t.File, ":")
			var filekey, filename string
			if len(kv) == 2 {
				filekey = "file"
				filename = kv[1]
			} else if len(kv) == 3 {
				filekey = kv[1]
				filename = kv[2]
			} else {
				return nil, fmt.Errorf("Form file: "+"(%s): illegal", t.File)
			}
			fw, err := w.CreateFormFile(filekey, filename)
			if err != nil {
				return nil, err
			}
			fd, err := os.Open(filename)
			if err != nil {
				return nil, err
			}
			defer fd.Close()
			_, err = io.Copy(fw, fd)
			if err != nil {
				return nil, err
			}
			w.Close()
			req, err = http.NewRequest(t.Method, t.URL, buf)
			req.Header.Set("Content-Type", w.FormDataContentType())
		} else {
			bodyr, err := os.Open(t.File)
			if err != nil {
				return nil, fmt.Errorf("Post file: "+"(%s): %s", t.File, err)
			}
			defer bodyr.Close()
			var body []byte
			if body, err = ioutil.ReadAll(bodyr); err != nil {
				return nil, fmt.Errorf("Post file: "+"(%s): %s", t.File, err)
			}
			req, err = http.NewRequest(t.Method, t.URL, bytes.NewBuffer(body))
			contentLen := len(body)
			req.Header.Set("Content-Length", fmt.Sprint(contentLen))
		}
	} else {
		req, err = http.NewRequest(t.Method, t.URL, bytes.NewBuffer(t.Body))
	}

	if err != nil {
		return nil, err
	}
	for k, vs := range t.Header {
		req.Header[k] = make([]string, len(vs))
		copy(req.Header[k], vs)
	}
	req.Header.Set("User-Agent", "stress 1.0")
	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}
	return req, nil
}

func (t Targets) Shuffle(seed int64) {
	rand.Seed(seed)
	for i, rnd := range rand.Perm(len(t)) {
		t[i], t[rnd] = t[rnd], t[i]
	}
}
