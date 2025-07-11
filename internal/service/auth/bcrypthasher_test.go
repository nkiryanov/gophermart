package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)


func Test_BcryptHasher(t *testing.T) {
	t.Parallel()
	
	h := BcryptHasher{}

	t.Run("hash password", func(t *testing.T) {
		got, err := h.Hash("password")
		require.NoError(t, err)

		require.Len(t, got, 60, "bcrypt length is 60 letters as far as i know")
		require.Equal(t, "$2a$", got[:4], "bcrypt has should have prefix '$2a$'")
	})

	t.Run("compare password ok", func(t *testing.T) {
		hash, err := h.Hash("password")
		require.NoError(t, err)

		err = h.Compare(hash, "password")
		
		require.NoError(t, err)
	})

	t.Run("fail compare if wrong password", func(t *testing.T) {
		hash, err := h.Hash("password")
		require.NoError(t, err)

		err = h.Compare(hash, "wrong")
		
		require.Error(t, err)
	})
}
