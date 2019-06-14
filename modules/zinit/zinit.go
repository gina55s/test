// Package zinit exposes function to interat with zinit service life cyle management
package zinit

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const defaultSocketPath = "/var/run/zinit.sock"

type ZinitClient struct {
	socket string //path to the unix socket
	conn   net.Conn
}

func New(socket string) *ZinitClient {
	if socket == "" {
		socket = defaultSocketPath
	}
	return &ZinitClient{socket: socket}
}

func (c *ZinitClient) Connect() error {
	if c.conn != nil {
		return fmt.Errorf("already connected")
	}

	conn, err := net.Dial("unix", c.socket)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *ZinitClient) Close() error {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return err
		}
	}

	c.conn = nil
	return nil
}

func (c *ZinitClient) cmd(cmd string) (string, error) {
	if c.conn == nil {
		return "", fmt.Errorf("not connected, call Connect() before executing command ")
	}
	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		return "", err
	}
	if _, err := c.conn.Write([]byte("\n")); err != nil {
		return "", err
	}
	return c.readResponse()
}

func (c *ZinitClient) readResponse() (string, error) {
	var (
		count  uint64
		status string
		err    error
	)

	headers := map[string]string{}
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// end of headers section
			break
		}
		parts := strings.SplitN(line, ":", 2)
		headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])

	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error while reading socket: %v", err)
	}

	count, err = strconv.ParseUint(headers["lines"], 10, 32)
	if err != nil {
		return "", err
	}
	status = headers["status"]

	content := ""
	for i := uint64(0); i < count; i++ {
		if !scanner.Scan() {
			break
		}
		content += scanner.Text() + "\n"
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error while reading socket: %v", err)
	}

	if status == "error" {
		return "", fmt.Errorf(string(content))
	}

	return strings.TrimSpace(content), nil
}
