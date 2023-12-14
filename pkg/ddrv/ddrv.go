package ddrv

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type Driver struct {
	Rest      *Rest
	ChunkSize int
}

type Config struct {
	Token      string
	TokenType  int
	Channels   string
	AsyncWrite bool
	ChunkSize  int
}

func New(cfg *Config) (*Driver, error) {
	if cfg.Token == "" || cfg.Channels == "" {
		return nil,
			fmt.Errorf("channels or token is missing: token = %s channels = %s", cfg.Token, cfg.Channels)
	}
	tokens := strings.Split(cfg.Token, ",")
	channels := strings.Split(cfg.Channels, ",")
	chunkSize, err := parseChunkSize(cfg.ChunkSize, cfg.TokenType)
	if err != nil {
		return nil, err
	}
	nitro := false
	if chunkSize > 100*1024*1024 && cfg.TokenType == TokenUserNitro {
		nitro = true
	}
	for i, token := range tokens {
		if cfg.TokenType == TokenBot {
			tokens[i] = "Bot " + token
		}
	}
	return &Driver{NewRest(tokens, channels, nitro), chunkSize}, nil
}

// NewWriter creates a new ddrv.Writer instance that implements an io.WriterCloser.
// This allows for writing large files to Discord as small, manageable chunks.
func (d *Driver) NewWriter(onChunk func(chunk Node)) io.WriteCloser {
	return NewWriter(onChunk, d.ChunkSize, d.Rest)
}

// NewNWriter creates a new ddrv.NWriter instance that implements an io.WriterCloser.
// This allows for writing large files to Discord as small, manageable chunks.
// NWriter buffers bytes into memory and writes data to discord in parallel
func (d *Driver) NewNWriter(onChunk func(chunk Node)) io.WriteCloser {
	return NewNWriter(onChunk, d.ChunkSize, d.Rest)
}

// NewReader creates a new Reader instance that implements an io.ReaderCloser.
// This allows for reading large files from Discord that were split into small chunks.
func (d *Driver) NewReader(chunks []Node, pos int64) (io.ReadCloser, error) {
	return NewReader(chunks, pos, d.Rest)
}

// UpdateNodes finds expired chunks and updates chunk signature in given chunks slice
func (d *Driver) UpdateNodes(chunks []*Node) error {
	currentTimestamp := int(time.Now().Unix())
	expired := make(map[int64]*Node)

	for i, chunk := range chunks {
		if currentTimestamp > chunk.Ex {
			expired[chunk.MId] = chunks[i]
		}
	}

	var messages []Message
	for mid, chunk := range expired {
		if currentTimestamp > chunk.Ex {
			cid := extractChannelId(chunk.URL)
			fmt.Println(cid)
			if err := d.Rest.GetMessages(cid, mid-1, "after", &messages); err != nil {
				return err
			}
			for _, msg := range messages {
				id, _ := strconv.ParseInt(msg.Id, 10, 64)
				if updatedChunk, ok := expired[id]; ok {
					updatedChunk.URL, updatedChunk.Ex, updatedChunk.Is, updatedChunk.Hm = DecodeAttachmentURL(msg.Attachments[0].URL)
				}
			}
		}
	}
	return nil
}

// parseChunkSize is a function that accepts a size and a tokenType as its arguments.
// It returns an adjusted chunkSize and an error if the provided chunkSize is invalid.
func parseChunkSize(chunkSize, tokenType int) (int, error) {
	// Check if provided token is valid
	if tokenType > TokenUserNitroBasic {
		return 0, fmt.Errorf("invalid token type %d", tokenType)
	}
	// If the tokenType is either TokenBot or TokenUser and if chunkSize is greater than 25MB, adjust chunkSize to 25MB.
	if (tokenType == TokenBot || tokenType == TokenUser) && (chunkSize > 25*1024*1024 || chunkSize <= 0) {
		chunkSize = 25 * 1024 * 1024
	}
	// If the tokenType is TokenUserNitroBasic and chunkSize is greater than 50MB, adjust chunkSize to 50MB.
	if tokenType == TokenUserNitroBasic && (chunkSize > 50*1024*1024 || chunkSize <= 0) {
		chunkSize = 50 * 1024 * 1024
	}
	// If the tokenType is TokenUserNitro and chunkSize is greater than 500MB, adjust chunkSize to 500MB.
	if tokenType == TokenUserNitro && (chunkSize > 500*1024*1024 || chunkSize <= 0) {
		chunkSize = 500 * 1024 * 1024
	}
	// Return the adjusted chunkSize and nil as there is no error.
	return chunkSize, nil
}
