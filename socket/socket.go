package socket

import (
	"bufio"
	"context"
	"encoding/binary"
	"github.com/brodyxchen/vsock/constant"
	"github.com/brodyxchen/vsock/errors"
	"github.com/brodyxchen/vsock/log"
	"github.com/brodyxchen/vsock/models"
	"io"
	"math"
	"time"
)

func ReadSocket(key string, ctx context.Context, reader *bufio.Reader) (*models.Header, []byte, error) {
	select {
	case <-ctx.Done():
		return nil, nil, errors.ErrCtxReadDone
	default:
	}

	header := &models.Header{}

	headerBuf := make([]byte, models.HeaderSize)
	n, err := io.ReadFull(reader, headerBuf)
	if err != nil {
		if err == io.EOF {
			log.Errorf("%v.readSocket() read header err == io.EOF, n=%v\n", key, n)
			return nil, nil, io.ErrUnexpectedEOF
		}
		log.Errorf("%v.readSocket() read header err != nil: %v\n", key, err)
		return nil, nil, err
	}
	if n < models.HeaderSize {
		log.Errorf("server.readSocket() read header n(%v) < HeaderSize\n", n)
		return nil, nil, errors.ErrInvalidHeader
	}

	header.Magic = binary.BigEndian.Uint16(headerBuf[:])
	if header.Magic != constant.DefaultMagic {
		log.Errorf("server.readSocket() header.Magic(%v) != defaultMagic\n", header.Magic)
		return nil, nil, errors.ErrInvalidHeaderMagic
	}

	header.Version = binary.BigEndian.Uint16(headerBuf[2:])
	header.Code = binary.BigEndian.Uint16(headerBuf[4:])
	header.Length = binary.BigEndian.Uint16(headerBuf[6:])

	if header.Length <= 0 {
		return header, nil, nil
	}

	bodyBuf := make([]byte, header.Length)
	n, err = io.ReadFull(reader, bodyBuf)
	if err != nil {
		if err == io.EOF {
			return header, nil, io.ErrUnexpectedEOF
		}
		return nil, nil, err
	}

	if n < int(header.Length) {
		return nil, nil, errors.ErrInvalidBody
	}

	return header, bodyBuf, err
}

func ReadSocketTest(key string, waitTime time.Duration, ctx context.Context, reader *bufio.Reader) (*models.Header, []byte, error) {
	select {
	case <-ctx.Done():
		return nil, nil, errors.ErrCtxReadDone
	default:
	}

	header := &models.Header{}

	headerBuf := make([]byte, models.HeaderSize)
	n, err := io.ReadFull(reader, headerBuf)
	if err != nil {
		if err == io.EOF {
			log.Errorf("%v.readSocketTest() read header err == io.EOF, n=%v, gapTime=%v\n", key, n, waitTime)
			return nil, nil, io.ErrUnexpectedEOF
		}
		log.Errorf("%v.readSocketTest() read header err != nil: %v,  gapTime=%v\n", key, err, waitTime)
		return nil, nil, err
	}
	if n < models.HeaderSize {
		log.Errorf("server.readSocketTest() read header n(%v) < HeaderSize\n", n)
		return nil, nil, errors.ErrInvalidHeader
	}

	header.Magic = binary.BigEndian.Uint16(headerBuf[:])
	if header.Magic != constant.DefaultMagic {
		log.Errorf("server.readSocketTest() header.Magic(%v) != defaultMagic\n", header.Magic)
		return nil, nil, errors.ErrInvalidHeaderMagic
	}

	header.Version = binary.BigEndian.Uint16(headerBuf[2:])
	header.Code = binary.BigEndian.Uint16(headerBuf[4:])
	header.Length = binary.BigEndian.Uint16(headerBuf[6:])

	if header.Length <= 0 {
		return header, nil, nil
	}

	bodyBuf := make([]byte, header.Length)
	n, err = io.ReadFull(reader, bodyBuf)
	if err != nil {
		if err == io.EOF {
			return header, nil, io.ErrUnexpectedEOF
		}
		return nil, nil, err
	}

	if n < int(header.Length) {
		return nil, nil, errors.ErrInvalidBody
	}

	return header, bodyBuf, err
}

func WriteSocket(ctx context.Context, writer *bufio.Writer, header *models.Header, body []byte) error {
	select {
	case <-ctx.Done():
		return errors.ErrCtxWriteDone
	default:
	}

	length := len(body)
	if length > math.MaxUint16 {
		return errors.ErrExceedBody
	}
	header.Length = uint16(length)

	buf := make([]byte, models.HeaderSize+length)
	binary.BigEndian.PutUint16(buf, header.Magic)
	binary.BigEndian.PutUint16(buf[2:], header.Version)
	binary.BigEndian.PutUint16(buf[4:], header.Code)
	binary.BigEndian.PutUint16(buf[6:], header.Length)
	if length > 0 {
		copy(buf[models.HeaderSize:], body)
	}

	_, err := writer.Write(buf)
	if err != nil {
		return err
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	return nil
}
