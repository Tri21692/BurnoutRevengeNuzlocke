package main

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"time"
)

// Pine talks to PCSX2's PINE interface (needed to write to pages holding translated code).
type Pine struct{ c net.Conn }

func dialPine() (*Pine, error) {
	c, err := net.DialTimeout("tcp", "127.0.0.1:28011", 2*time.Second)
	if err != nil {
		return nil, err
	}
	return &Pine{c: c}, nil
}

func (p *Pine) cmd(payload []byte) ([]byte, error) {
	_ = p.c.SetDeadline(time.Now().Add(3 * time.Second))
	msg := make([]byte, 4, 4+len(payload))
	binary.LittleEndian.PutUint32(msg, uint32(len(payload)+4))
	if _, err := p.c.Write(append(msg, payload...)); err != nil {
		return nil, err
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(p.c, head); err != nil {
		return nil, err
	}
	body := make([]byte, binary.LittleEndian.Uint32(head)-4)
	if _, err := io.ReadFull(p.c, body); err != nil {
		return nil, err
	}
	if len(body) == 0 || body[0] != 0 {
		return nil, errors.New("PINE command failed")
	}
	return body[1:], nil
}

func (p *Pine) Write32(addr, v uint32) error {
	b := []byte{0x06, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(b[1:], addr)
	binary.LittleEndian.PutUint32(b[5:], v)
	_, err := p.cmd(b)
	return err
}

func (p *Pine) Write8(addr uint32, v byte) error {
	b := []byte{0x04, 0, 0, 0, 0, v}
	binary.LittleEndian.PutUint32(b[1:], addr)
	_, err := p.cmd(b)
	return err
}

func (p *Pine) Serial() string {
	d, err := p.cmd([]byte{0x0C})
	if err != nil || len(d) < 4 {
		return ""
	}
	n := int(binary.LittleEndian.Uint32(d))
	if 4+n > len(d) {
		n = len(d) - 4
	}
	return strings.TrimRight(string(d[4:4+n]), "\x00")
}

func (p *Pine) WriteBytes(addr uint32, data []byte) error {
	i := 0
	for ; i+4 <= len(data); i += 4 {
		if err := p.Write32(addr+uint32(i), binary.LittleEndian.Uint32(data[i:])); err != nil {
			return err
		}
	}
	for ; i < len(data); i++ {
		if err := p.Write8(addr+uint32(i), data[i]); err != nil {
			return err
		}
	}
	return nil
}

func (p *Pine) Close() { _ = p.c.Close() }
