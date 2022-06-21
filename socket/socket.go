package socket

import (
	"bufio"
	"context"
	"encoding/binary"
	"github.com/brodyxchen/vsock-sdk/constant"
	"github.com/brodyxchen/vsock-sdk/errors"
	"github.com/brodyxchen/vsock-sdk/models"
	"io"
	"math"
)

func ReadSocket(ctx context.Context, reader *bufio.Reader) (*models.Header, []byte, bool, error) {
	select {
	case <-ctx.Done():
		return nil, nil, false, errors.ErrCtxReadDone
	default:
	}

	header := &models.Header{}

	headerBuf := make([]byte, models.HeaderSize)
	n, err := io.ReadFull(reader, headerBuf)
	if err != nil {
		if err == io.EOF {
			return nil, nil, true, io.ErrUnexpectedEOF
		}
		return nil, nil, true, err
	}
	if n < models.HeaderSize {
		return nil, nil, false, errors.ErrInvalidHeader
	}

	header.Magic = binary.BigEndian.Uint16(headerBuf[:])
	if header.Magic != constant.DefaultMagic {
		return nil, nil, false, errors.ErrInvalidHeaderMagic
	}

	header.Version = binary.BigEndian.Uint16(headerBuf[2:])
	header.Code = binary.BigEndian.Uint16(headerBuf[4:])
	header.Length = binary.BigEndian.Uint16(headerBuf[6:])

	if header.Length <= 0 {
		return header, nil, false, nil
	}

	bodyBuf := make([]byte, header.Length)
	n, err = io.ReadFull(reader, bodyBuf)
	if err != nil {
		if err == io.EOF {
			return header, nil, true, io.ErrUnexpectedEOF
		}
		return nil, nil, true, err
	}

	if n < int(header.Length) {
		return nil, nil, false, errors.ErrInvalidBody
	}

	return header, bodyBuf, false, nil
}

func WriteSocket(ctx context.Context, writer *bufio.Writer, header *models.Header, body []byte) (bool, error) {
	select {
	case <-ctx.Done():
		return false, errors.ErrCtxWriteDone
	default:
	}

	length := len(body)
	if length > math.MaxUint16 {
		return false, errors.ErrExceedBody
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
		return true, err
	}

	err = writer.Flush()
	if err != nil {
		return true, err
	}

	return false, nil
}
