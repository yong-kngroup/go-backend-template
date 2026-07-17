// Package crypto 提供密码加密相关的适配器实现。
package crypto

import "golang.org/x/crypto/bcrypt"

// BcryptHasher 基于 bcrypt 提供密码哈希与校验能力。
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher 创建 bcrypt 密码加密器。
// cost 为 0 时使用 bcrypt.DefaultCost(10)。
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (h *BcryptHasher) Verify(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
