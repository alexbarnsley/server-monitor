package main

import (
	"bytes"
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"
)

type sshSession struct {
	client *ssh.Client
	server *ServerConfig
}

func sshConnect(server *ServerConfig) (*sshSession, error) {
	hostKey := ssh.InsecureIgnoreHostKey()
	config := &ssh.ClientConfig{
		User: server.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(server.Password),
		},
		HostKeyCallback: hostKey,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", server.Host, server.Port), config)
	if err != nil {
		return nil, errors.New("Could not connect to " + server.Host + ": " + err.Error())
	}

	return &sshSession{
		client: client,
		server: server,
	}, nil
}

func (serverSession *sshSession) RunCommand(command string) (*bytes.Buffer, error) {
	session, err := serverSession.client.NewSession()
	if err != nil {
		return nil, errors.New("Could not start session for " + serverSession.server.Host + ": " + err.Error())
	}

	var response bytes.Buffer
	session.Stdout = &response
	if err := session.Run(command); err != nil {
		return nil, err
	}

	return &response, nil
}
