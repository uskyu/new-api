package common

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBodyStorageNewReaderIndependentMemory(t *testing.T) {
	payload := []byte("independent-memory-readers")
	storage := newMemoryStorage(payload)
	t.Cleanup(func() { _ = storage.Close() })

	assertIndependentBodyStorageReaders(t, storage, payload)

	require.NoError(t, storage.Close())
	_, err := storage.NewReader()
	require.ErrorIs(t, err, ErrStorageClosed)
}

func TestBodyStorageNewReaderIndependentDisk(t *testing.T) {
	previousConfig := GetDiskCacheConfig()
	SetDiskCacheConfig(DiskCacheConfig{
		Enabled:     true,
		ThresholdMB: 0,
		MaxSizeMB:   1,
		Path:        t.TempDir(),
	})
	t.Cleanup(func() { SetDiskCacheConfig(previousConfig) })

	payload := []byte("independent-disk-readers")
	storage, err := newDiskStorage(payload, GetDiskCachePath())
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })

	assertIndependentBodyStorageReaders(t, storage, payload)

	require.NoError(t, storage.Close())
	_, err = storage.NewReader()
	require.ErrorIs(t, err, ErrStorageClosed)
}

func assertIndependentBodyStorageReaders(t *testing.T, storage BodyStorage, payload []byte) {
	t.Helper()

	first, err := storage.NewReader()
	require.NoError(t, err)
	defer first.Close()

	prefix := make([]byte, 5)
	_, err = io.ReadFull(first, prefix)
	require.NoError(t, err)
	require.Equal(t, payload[:5], prefix)

	second, err := storage.NewReader()
	require.NoError(t, err)
	defer second.Close()

	secondBody, err := io.ReadAll(second)
	require.NoError(t, err)
	require.Equal(t, payload, secondBody)

	firstRemainder, err := io.ReadAll(first)
	require.NoError(t, err)
	require.Equal(t, payload[5:], firstRemainder)

	_, err = storage.Seek(0, io.SeekStart)
	require.NoError(t, err)
	storageBody, err := io.ReadAll(storage)
	require.NoError(t, err)
	require.Equal(t, payload, storageBody)
}

func TestBodyStorageNewReaderClosedError(t *testing.T) {
	storage := newMemoryStorage([]byte("closed"))
	require.NoError(t, storage.Close())

	reader, err := storage.NewReader()
	require.Nil(t, reader)
	require.True(t, errors.Is(err, ErrStorageClosed))
}
