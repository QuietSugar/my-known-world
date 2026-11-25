package bitwarden

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// —— 解密链路 ——
//
// 这几条错了的表现都是「连得上、拉得到、解出来是乱码或报 MAC 失败」，
// 光看日志分不清是哪一步。所以每一步单独钉住。

// enc 造一条 Bitwarden 格式的密文（用来测解密）
func enc(t *testing.T, k encKey, plain string) string {
	t.Helper()
	return makeEncString(t, k, plain)
}

// makeEncString 按 `2.iv|data|mac` 拼一条密文
//
// **故意不调用被测代码**：用测试自己实现的加密端，才能发现被测的解密端
// 写反了（两边共用一份加密代码时，反了也照样「通过」）。
func makeEncString(t *testing.T, k encKey, plain string) string {
	t.Helper()
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i)
	}
	// PKCS#7 填充
	pad := 16 - len(plain)%16
	data := append([]byte(plain), []byte(strings.Repeat(string(rune(pad)), pad))...)

	out := make([]byte, len(data))
	cbcEncrypt(t, k.enc, iv, out, data)

	h := hmac.New(sha256.New, k.mac)
	h.Write(iv)
	h.Write(out)
	mac := h.Sum(nil)

	return "2." + base64.StdEncoding.EncodeToString(iv) + "|" +
		base64.StdEncoding.EncodeToString(out) + "|" +
		base64.StdEncoding.EncodeToString(mac)
}

// TestDecryptRoundTrip 密文能解回原文
func TestDecryptRoundTrip(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	k, err := newEncKey(raw)
	if err != nil {
		t.Fatal(err)
	}

	for _, plain := range []string{"hello", "中文也要能解", "", strings.Repeat("x", 100)} {
		got, err := k.decryptStr(enc(t, k, plain))
		if err != nil {
			t.Fatalf("解 %q 失败: %v", plain, err)
		}
		if got != plain {
			t.Fatalf("解出来是 %q，期望 %q", got, plain)
		}
	}
}

// TestDecryptPlaintextPassesThrough 不是密文的字段原样返回
//
// 有些字段本来就存明文；把它当「解不开」而报错，等于一半账号用不了。
func TestDecryptPlaintextPassesThrough(t *testing.T) {
	k, _ := newEncKey(make([]byte, 32))
	got, err := k.decryptStr("就是明文")
	if err != nil || got != "就是明文" {
		t.Fatalf("明文应原样返回，得到 %q err=%v", got, err)
	}
}

// TestDecryptRejectsTamperedCiphertext MAC 不对必须失败
//
// 不验 MAC 直接解，等于让伪造的密文进入解密器；CBC 对填充错误的反应
// 可以被用来判断内容，验 MAC 是唯一可靠的防线。
func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	raw := make([]byte, 32)
	k, _ := newEncKey(raw)

	s := enc(t, k, "secret")
	parts := strings.Split(s[2:], "|")
	// 改数据段
	data, _ := base64.StdEncoding.DecodeString(parts[1])
	data[0] ^= 0xff
	parts[1] = base64.StdEncoding.EncodeToString(data)

	if _, err := k.decryptStr("2." + strings.Join(parts, "|")); err == nil {
		t.Fatal("密文被改动应报错")
	}
}

// TestDeriveMasterKeyEmailCaseInsensitive email 大小写不该影响密钥
//
// Bitwarden 客户端一律按小写 email 派生；大小写不同会解不开同一个账号。
func TestDeriveMasterKeyEmailCaseInsensitive(t *testing.T) {
	a, err := deriveMasterKey("pw", "A@B.C", 1000)
	if err != nil {
		t.Fatal(err)
	}
	b, err := deriveMasterKey("pw", "a@b.c", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("email 大小写不应影响派生结果")
	}

	c, _ := deriveMasterKey("pw2", "a@b.c", 1000)
	if string(a) == string(c) {
		t.Fatal("密码不同应得到不同密钥")
	}
	// 迭代次数不同也应不同
	d, _ := deriveMasterKey("pw", "a@b.c", 2000)
	if string(a) == string(d) {
		t.Fatal("迭代次数不同应得到不同密钥")
	}
}

// TestParseEncString 格式识别
func TestParseEncString(t *testing.T) {
	e, ok := parseEncString("2.aGVsbG8=aGVsbG8=")
	if ok {
		t.Fatal("缺少分隔符应识别失败")
	}

	e, ok = parseEncString("2.aGVsbG8=|d29ybGQ=|bWFj")
	if !ok {
		t.Fatal("标准格式应识别成功")
	}
	if e.Type != 2 || !e.HasMAC || string(e.IV) != "hello" || string(e.Data) != "world" {
		t.Fatalf("解析结果不对: %+v", e)
	}

	if _, ok := parseEncString("x.y|z"); ok {
		t.Fatal("类型不是数字应识别失败")
	}
}

// TestSplitProfileKey 新版 profile.key 是两段拼在一起
func TestSplitProfileKey(t *testing.T) {
	one := "2.iv|data|mac"
	if k, ty := splitProfileKey(one); k != one || ty != "" {
		t.Fatalf("单段格式不该被拆开: %q / %q", k, ty)
	}

	two := "2.iv1|data1|mac1|2.iv2|data2|mac2"
	k, ty := splitProfileKey(two)
	if k != "2.iv1|data1|mac1" {
		t.Fatalf("前半段拆错: %q", k)
	}
	if ty != "2.iv2|data2|mac2" {
		t.Fatalf("后半段拆错: %q", ty)
	}
}

// cbcEncrypt 测试自用的 CBC 加密（与生产的解密端各自独立）
func cbcEncrypt(t *testing.T, key, iv, dst, src []byte) {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(dst, src)
}

// TestHKDFExpandMatchesRFC5869 用 RFC 5869 的官方测试向量钉住 Expand
//
// 自己写的 HKDF 最容易错在「只算一块」：输出超过 32 字节时要按 T(1)|info|计数 递增。
// 用标准向量比「和另一个实现比」更可靠——后者可能一起错。
func TestHKDFExpandMatchesRFC5869(t *testing.T) {
	// RFC 5869 Test Case 1：PRK 与 info 都是十六进制常量
	prk := mustHex(t, "077709362c2e32df0ddc3f0dc47bba6390b6c73bb50f9c3122ec844ad7c2b3e5")
	info := mustHex(t, "f0f1f2f3f4f5f6f7f8f9")
	want := mustHex(t, "3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865")

	got := hkdfExpand(prk, info, 42)
	if string(got) != string(want) {
		t.Fatalf("HKDF-Expand 结果不对\n得到 %x\n期望 %x", got, want)
	}
}

// TestStretchMasterKeyLayout 派生结果是 enc(32)+mac(32)，共 64 字节
func TestStretchMasterKeyLayout(t *testing.T) {
	k := stretchMasterKey(make([]byte, 32))
	if len(k) != 64 {
		t.Fatalf("应为 64 字节，实际 %d", len(k))
	}
	// 前 32 与后 32 必须不同（info 不同），否则说明两块都用了同一个 info
	if string(k[:32]) == string(k[32:]) {
		t.Fatal("enc 与 mac 两段不该相同")
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestClassifyErrorKinds 四类失败要能区分
//
// 「主密码错」「连不上」「属性没配」「本应用坏了」这四种对使用者来说
// 是完全不同的四件事：去改属性、去查网络、去补配置、去看日志。
// 只给一句错误文本，人得逐字读才能判断该动哪里。
func TestClassifyErrorKinds(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"主密码错", errOf(errKindAuth, "改主密码", "x"), errKindAuth},
		{"网络", errOf(errKindNetwork, "查地址", "x"), errKindNetwork},
		{"配置", errOf(errKindConfig, "补配置", "x"), errKindConfig},
		{"上游", errOf(errKindUpstream, "重试", "x"), errKindUpstream},
	}
	for _, c := range cases {
		got, _ := classify(c.err)
		if got != c.want {
			t.Errorf("%s: 分类 = %q，期望 %q", c.name, got, c.want)
		}
	}

	// 没打标签的错误落到 internal，而不是硬塞进某个人为分类
	if got, _ := classify(errOf("", "", "裸错误")); got != errKindInternal {
		t.Errorf("未分类错误应落到 internal，实际 %q", got)
	}
}

// TestDecryptItemSkipsOnFailure 单条解不开只影响它自己
//
// 曾经是整次同步失败：库里只要有一条解不开，其余几百条也同步不了。
// 但也不能静默——返回的原因要能进 skipped，否则「查不到」会被
// 当成「本来就没有」。
func TestDecryptItemSkipsOnFailure(t *testing.T) {
	raw := make([]byte, 32)
	good, _ := newEncKey(raw)

	// 用另一把密钥加密，good 解不开
	otherRaw := make([]byte, 32)
	for i := range otherRaw {
		otherRaw[i] = 0xff
	}
	other, _ := newEncKey(otherRaw)

	it := cipherItem{
		ID:           "bad",
		RevisionDate: "2026-01-01T00:00:00Z",
		Name:         makeEncString(t, other, "秘密名称"),
	}

	item, reason := decryptItem(good, it)
	if reason == "" {
		t.Fatal("解不开时应返回原因")
	}
	if item.ID != "" {
		t.Error("失败时不该返回半截条目")
	}

	// 正常的条目不受影响
	ok := cipherItem{ID: "ok", Name: makeEncString(t, good, "名字")}
	if item, reason := decryptItem(good, ok); reason != "" || item.Name != "名字" {
		t.Fatalf("正常条目应解开，得到 %+v %q", item, reason)
	}
}

// TestDecryptItemSingleBadUriKeepsItem 单个 URL 解不开不毁整条
func TestDecryptItemSingleBadUriKeepsItem(t *testing.T) {
	raw := make([]byte, 32)
	k, _ := newEncKey(raw)
	otherRaw := make([]byte, 32)
	for i := range otherRaw {
		otherRaw[i] = 0xff
	}
	other, _ := newEncKey(otherRaw)

	var c cipherItem
	c.ID = "x"
	c.Name = makeEncString(t, k, "名字")
	c.Login = &struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Uris     []struct {
			URI string `json:"uri"`
		} `json:"uris"`
	}{}
	c.Login.Password = makeEncString(t, k, "pw")
	c.Login.Uris = append(c.Login.Uris, struct {
		URI string `json:"uri"`
	}{URI: makeEncString(t, other, "坏uri")})
	c.Login.Uris = append(c.Login.Uris, struct {
		URI string `json:"uri"`
	}{URI: makeEncString(t, k, "good.example")})

	item, reason := decryptItem(k, c)
	if reason != "" {
		t.Fatalf("单条 URL 坏掉不该让整条失败: %s", reason)
	}
	if item.Password != "pw" {
		t.Error("密码应仍被解出")
	}
	if len(item.Uris) != 1 || item.Uris[0] != "good.example" {
		t.Errorf("应只保留解得开的 URL，实际 %v", item.Uris)
	}
}
