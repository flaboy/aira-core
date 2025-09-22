package hashid

import (
	"errors"
	"strings"
	"sync"

	"github.com/flaboy/aira/aira-core/pkg/config"
	"github.com/speps/go-hashids/v2"
)

var sequenceMap = map[string]*hashtype{}
var lk sync.Mutex

type hashtype struct {
	prefix    string
	prefixLen int
	data      *hashids.HashIDData
}

func getPrefix(hashid *HashID) string {
	return hashid.Short
}

func newHashType(hashid *HashID) *hashtype {
	data := hashids.NewData()
	data.Salt = config.Config.AppSecret + hashid.Name
	data.MinLength = hashid.MinLen
	if data.MinLength <= 0 {
		data.MinLength = 4
	}
	data.Alphabet = "abcdefghijklmnopqrstuvwxyz1234567890"
	prefix := getPrefix(hashid)
	return &hashtype{
		prefix:    prefix,
		prefixLen: len(prefix),
		data:      data,
	}
}

func getSequenceHd(hashid *HashID) *hashtype {
	lk.Lock()
	defer lk.Unlock()
	key := hashid.Short
	if _, exists := sequenceMap[key]; !exists {
		sequenceMap[key] = newHashType(hashid)
	}
	return sequenceMap[key]
}

func Encode(hashid *HashID, id uint) string {
	pd := getSequenceHd(hashid)
	hd, err := hashids.NewWithData(pd.data)
	if err != nil {
		return ""
	}
	e, err := hd.Encode([]int{int(id)})
	if err != nil {
		return ""
	}
	return pd.prefix + e
}

func Decode(hashid *HashID, hash_id string) (uint, error) {
	pd := getSequenceHd(hashid)
	if !strings.HasPrefix(hash_id, pd.prefix) {
		return 0, errors.New(hashid.Name + " id does not start with the expected prefix: " + pd.prefix)
	}
	if len(hash_id) <= pd.prefixLen {
		return 0, errors.New(hashid.Name + " id is too short")
	}
	hd, err := hashids.NewWithData(pd.data)
	if err != nil {
		return 0, errors.New("Failed to get " + hashid.Name + " id decoder: " + err.Error())
	}
	e, err := hd.DecodeWithError(hash_id[pd.prefixLen:])
	if err != nil {
		return 0, errors.New("Failed to decode " + hashid.Name + " id: " + err.Error())
	}
	if len(e) == 0 {
		return 0, errors.New(hashid.Name + " id is invalid")
	}
	return uint(e[0]), nil
}
