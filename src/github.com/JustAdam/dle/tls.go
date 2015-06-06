// Copyright 2014 JustAdam (adambell7@gmail.com).  All rights reserved.
// License: MIT

// Docker log file client for logentries.
package main

import (
	"crypto/tls"
	"crypto/x509"
	log "github.com/Sirupsen/logrus"
	"io"
	"io/ioutil"
	"time"
)

type TLSConnection struct {
	host     string
	certPool *x509.CertPool
	conn     *tls.Conn
}

func NewTLSConnection(host string) (*TLSConnection, error) {
	c := &TLSConnection{}
	c.host = host
	c.getCerts()
	err := c.Connect()
	return c, err
}

func (c *TLSConnection) Connect() (err error) {
	c.conn, err = tls.Dial("tcp", c.host, &tls.Config{
		RootCAs: c.certPool,
	})
	if err != nil {
		return
	}
	c.conn.SetWriteDeadline(time.Time{})
	return
}

func (c *TLSConnection) WriteString(s string) (n int, err error) {
	return io.WriteString(c.conn, s)
}

func (c *TLSConnection) Write(p []byte) (n int, err error) {
	return c.conn.Write(p)
}

func (c *TLSConnection) Close() error {
	return c.conn.Close()
}

func (c *TLSConnection) getCerts() {
	c.certPool = x509.NewCertPool()
	cert, err := ioutil.ReadFile(certsPemFile)
	if err != nil {
		log.Fatal(err)
	}
	if !c.certPool.AppendCertsFromPEM(cert) {
		log.Fatal("Failed parsing root certificate")
	}
}
