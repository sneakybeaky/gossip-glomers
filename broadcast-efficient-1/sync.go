package main

import "log/slog"

type syncer struct {
	dest   string
	body   any
	logger *slog.Logger
	send   func(dest string, body any) error
}

func (s syncer) sync() error {

	s.logger.Debug("Sending sync to neighbour")

	err := s.send(s.dest, s.body)

	if err != nil {
		return err
	}

	return nil
}
