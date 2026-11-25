package bitwarden

// Bitwarden 的条目密文解密
//
// 这是「用 API Key 拿数据」绕不开的一步：/api/sync 返回的字段全是 EncString，
// Client ID/Secret 只解决「能不能读到」，读到的是什么还得靠主密码解开。
//
// 密钥链（Bitwarden 官方口径）：
//
//	masterKey = PBKDF2-SHA256(主密码, 小写 email, iterations)   // 32 字节
//	stretched = HKDF-Expand(masterKey, "enc", 32)              // 用来解 profile.key
//	userKey   = AES-256-CBC 解密 profile.key（MAC 用同一把 stretched 密钥）
//	条目字段   = AES-256-CBC 解密，密钥由 userKey 派生出 enc / mac 两把
//
// 只支持 kdfType=0（PBKDF2）。Argon2id（kdfType=1）需要额外依赖，
// 遇到时明确报错而不是解出垃圾——报错说清「暂不支持」，比静默返回密文强。

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// encString Bitwarden 的加密字符串：`<type>.<iv>|<数据>|<mac>`
//
//	type 2 = AesCbc256_HmacSha256（当前默认）
//	type 0 = AesCbc256（无 MAC，老数据）
type encString struct {
	Type   int
	IV     []byte
	Data   []byte
	MAC    []byte
	HasMAC bool
}

// parseEncString 解析 `2.xxx|yyy|zzz` 形式
//
// 不是这个形式（比如明文）时返回 ok=false：有些字段本来就存明文，
// 不该因为「解不开」就当成错误。
func parseEncString(s string) (encString, bool) {
	var out encString
	dot := strings.Index(s, ".")
	if dot <= 0 || dot > 2 {
		return out, false
	}
	t, err := strconv.Atoi(s[:dot])
	if err != nil {
		return out, false
	}
	out.Type = t

	parts := strings.Split(s[dot+1:], "|")
	if len(parts) < 2 {
		return out, false
	}
	out.IV, err = base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return out, false
	}
	out.Data, err = base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return out, false
	}
	if len(parts) >= 3 && parts[2] != "" {
		out.MAC, err = base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			return out, false
		}
		out.HasMAC = true
	}
	return out, true
}

// encKey 一次 AES-CBC 加解密要用的两把密钥
//
// 两种来源，长度不同，别搞混：
//
//	条目密钥（userKey）  32 字节拆两半：前 16 字节 AES 密钥、后 16 字节 HMAC 密钥
//	masterKey 派生结果   64 字节：前 32 字节 AES 密钥、后 32 字节 HMAC 密钥
//
// 所以这里不校验长度，由构造方（newEncKey / stretchMasterKey）各自负责。
type encKey struct {
	enc []byte
	mac []byte
}

func newEncKey(raw []byte) (encKey, error) {
	if len(raw) != 32 {
		return encKey{}, fmt.Errorf("密钥长度应为 32 字节，实际 %d", len(raw))
	}
	return encKey{enc: raw[:16], mac: raw[16:]}, nil
}

// decrypt 解一条 EncString
//
// **先验 MAC 再解密**：不验就直接解，等于让伪造的密文进入解密器，
// 而 CBC 对填充错误的反应可以被用来判断内容。验 MAC 失败即视为被改动。
func (k encKey) decrypt(e encString) (string, error) {
	if e.HasMAC {
		h := hmac.New(sha256.New, k.mac)
		h.Write(e.IV)
		h.Write(e.Data)
		want := h.Sum(nil)
		if subtle.ConstantTimeCompare(want, e.MAC) != 1 {
			return "", errors.New("MAC 校验失败（密文被改动或密钥不对）")
		}
	}
	block, err := aes.NewCipher(k.enc)
	if err != nil {
		return "", err
	}
	if len(e.Data)%aes.BlockSize != 0 {
		return "", errors.New("密文长度不是块大小的整数倍")
	}
	out := make([]byte, len(e.Data))
	cipher.NewCBCDecrypter(block, e.IV).CryptBlocks(out, e.Data)

	// 去 PKCS#7 填充
	if len(out) == 0 {
		return "", nil
	}
	pad := int(out[len(out)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(out) {
		return "", errors.New("填充不合法")
	}
	for _, b := range out[len(out)-pad:] {
		if int(b) != pad {
			return "", errors.New("填充不合法")
		}
	}
	return string(out[:len(out)-pad]), nil
}

// decryptStr 解一个字段；不是密文就原样返回（含空串）
func (k encKey) decryptStr(s string) (string, error) {
	e, ok := parseEncString(s)
	if !ok {
		return s, nil
	}
	return k.decrypt(e)
}

// ---------------------------------------------------------------- 密钥链

// preloginInfo 服务端在 /identity/accounts/prelogin 回的 KDF 参数
type preloginInfo struct {
	KDF           int `json:"kdf"`
	KDFIterations int `json:"kdfIterations"`
}

// deriveMasterKey 由主密码与 email 派生 masterKey
//
// email 要**转小写**：Bitwarden 客户端一律这么算，大小写不同会解不开。
func deriveMasterKey(password, email string, iterations int) ([]byte, error) {
	if iterations <= 0 {
		return nil, fmt.Errorf("kdfIterations 不合法: %d", iterations)
	}
	return pbkdf2.Key(sha256.New, password, []byte(strings.ToLower(strings.TrimSpace(email))), iterations, 32)
}

// hkdfExpand RFC 5869 的 Expand 步骤（Extract 那步在这里是恒等映射）
//
// 为什么要自己写这十几行：Bitwarden 的 masterKey 派生是
// `Hkdf::from_prk(masterKey).expand(info, out)`——**from_prk 表示 PRK 已经算好了**
// （就是 PBKDF2 的结果），只剩 Expand。用一个 HMAC 冒充它是不够的：
// 输出超过 32 字节时要拼 `T(1)|info|计数` 并按块递增，只算一块会得到错的密钥。
func hkdfExpand(prk, info []byte, length int) []byte {
	out := make([]byte, 0, length+sha256.Size)
	var block []byte
	for counter := byte(1); len(out) < length; counter++ {
		h := hmac.New(sha256.New, prk)
		h.Write(block)
		h.Write(info)
		h.Write([]byte{counter})
		block = h.Sum(nil)
		out = append(out, block...)
	}
	return out[:length]
}

// stretchMasterKey 派生「解 userKey 用的那把密钥」
//
// 与 Bitwarden 客户端一致：
//
//	prk       = PBKDF2(主密码, 小写 email, iterations)     ← 已经算好的 PRK
//	enc[0:32] = HKDF-Expand(prk, "enc", 32)
//	mac[32:64] = HKDF-Expand(prk, "mac", 32)
//
// 注意这里是 **enc 32 字节 + mac 32 字节**（共 64），不是把 32 字节切成两半——
// 这一点搞错的表现就是「能连上、能拉到数据，但 MAC 校验失败」。
func stretchMasterKey(masterKey []byte) []byte {
	out := make([]byte, 0, 64)
	out = append(out, hkdfExpand(masterKey, []byte("enc"), 32)...)
	out = append(out, hkdfExpand(masterKey, []byte("mac"), 32)...)
	return out
}

// splitProfileKey 把 profile.key 拆成「加密的 userKey」与「加密的类型标记」
//
// 新版账号的 profile.key 是把两个 EncString 拼在一起：
//
//	2.<iv>|<data>|<mac>|2.<iv>|<data>|<mac>
//	└── 加密的 userKey ──┘└── 加密的 userKey 类型 ──┘
//
// 老版只有前面那一段。这里的判据是「第二个 `|2.` 出现在中间」，而不是
// 数分隔符——MAC 本身就是 base64，里面不会有 `|`，但这个判据更不容易被将来的格式改动骗到。
func splitProfileKey(s string) (encUserKey string, encType string) {
	idx := strings.LastIndex(s, "|2.")
	if idx < 0 {
		return s, ""
	}
	// 前半截去掉末尾那个 `|`
	return s[:idx], s[idx+1:]
}

// ---------------------------------------------------------------- 用户密钥

// userKeySet 解开 profile.key 之后得到的用户密钥
//
// 同一份 userKey 同时用于：解条目字段、解组织密钥（后者本应用暂不需要）。
type userKeySet struct {
	entry encKey
}

// unlockUserKey 解 profile.key，得到解条目用的密钥
//
// profile.key 的格式随账号类型略有不同：
//   - `2.…` 单个 EncString：直接用 stretched 密钥解出 32 字节 userKey
//   - `2.…|2.…`（较新的「受保护的类型」）：前半是加密的 userKey，后半是加密的
//     userKey 的类型标记，解开后按标记决定是否还需要派生
func unlockUserKey(profileKey, password, email string, iterations int) (userKeySet, error) {
	masterKey, err := deriveMasterKey(password, email, iterations)
	if err != nil {
		return userKeySet{}, err
	}
	// stretched = enc(32) + mac(32)，正好是一次 encryptUserKey 要用的两把密钥
	stretched := stretchMasterKey(masterKey)
	key := encKey{enc: stretched[:32], mac: stretched[32:64]}

	rawKey, rawType := splitProfileKey(profileKey)
	e, ok := parseEncString(rawKey)
	if !ok {
		return userKeySet{}, fmt.Errorf("profile.key 格式不认识")
	}
	plain, err := key.decrypt(e)
	if err != nil {
		// 走到这里密钥派生参数已确定，解不开只剩「主密码不对」一种解释
		return userKeySet{}, errOf(errKindAuth,
			"主密码不对，在属性里重新填写", "解不开用户密钥（MAC 校验失败），主密码可能不正确")
	}
	userKey := []byte(plain)

	// 类型标记：1 = 32 字节（enc+mac），2 = 16 字节，需要自己扩展出 MAC 半段
	if rawType != "" {
		if te, ok := parseEncString(rawType); ok {
			if t, err := key.decrypt(te); err == nil {
				if strings.TrimSpace(t) == "2" && len(userKey) == 16 {
					h := hmac.New(sha256.New, userKey)
					h.Write([]byte("enc"))
					userKey = append(append([]byte{}, userKey...), h.Sum(nil)...)
				}
			}
		}
	}

	// userKey 的长度就是「两把密钥各自多长」：
	//   64 字节 = enc(32) + mac(32)  ← 当前默认（AesCbc256_HmacSha256）
	//   32 字节 = enc(16) + mac(16)  ← 老账号
	// 两种都要支持：按长度切，不按内容猜。
	var k encKey
	switch len(userKey) {
	case 64:
		k = encKey{enc: userKey[:32], mac: userKey[32:]}
	case 32:
		k = encKey{enc: userKey[:16], mac: userKey[16:]}
	default:
		return userKeySet{}, fmt.Errorf("userKey 长度异常：%d 字节（应为 32 或 64）", len(userKey))
	}
	return userKeySet{entry: k}, nil
}
