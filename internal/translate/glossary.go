package translate

// ruleEntry 是内置规则词典条目。
type ruleEntry struct {
	ZH     string // 规则中文名
	DescZH string // 规则描述中文
}

// builtinRules 是内置的规则名称/描述词典。
// 前 7 条来自真实审计结果 pyhackme.csv，做了精译；其余为常见 CodeQL 查询的通用译名，
// 覆盖不到的规则会走「模板 → SQLite 翻译库 → LLM」链路。
var builtinRules = map[string]ruleEntry{
	// ---- 实测数据中出现的规则（精译）----
	"Information exposure through an exception": {
		ZH:     "通过异常泄露信息",
		DescZH: "将异常信息（如错误消息、堆栈跟踪）泄露给外部用户，会暴露有助于攻击者策划后续利用的实现细节。",
	},
	"Full server-side request forgery": {
		ZH:     "完全的服务器端请求伪造（SSRF）",
		DescZH: "向完全由用户控制的 URL 发起网络请求，会被用于请求伪造攻击。",
	},
	"SQL query built from user-controlled sources": {
		ZH:     "使用用户可控数据拼接 SQL 查询",
		DescZH: "使用用户可控的数据源构建 SQL 查询，容易被注入恶意 SQL 代码。",
	},
	"Flask app is run in debug mode": {
		ZH:     "Flask 应用以调试模式运行",
		DescZH: "Flask 应用以调试模式运行，攻击者可能通过 Werkzeug 调试器执行任意代码。",
	},
	"Hard-coded credentials": {
		ZH:     "硬编码凭据",
		DescZH: "凭据被直接硬编码在应用程序的源码中。",
	},
	"Untrusted data passed to external API": {
		ZH:     "不可信数据传入外部 API",
		DescZH: "来自远端的不可信数据未经净化就被传入此外部 API，可能存在安全风险。",
	},
	"Reflected server-side cross-site scripting": {
		ZH:     "反射型服务端跨站脚本（XSS）",
		DescZH: "将用户输入直接写入网页，会造成跨站脚本（XSS）漏洞。",
	},

	// ---- 注入类 ----
	"Database query built from user-controlled sources": {ZH: "使用用户可控数据构建数据库查询"},
	"Code injection":                 {ZH: "代码注入"},
	"Command injection":              {ZH: "命令注入"},
	"Uncontrolled command line":      {ZH: "命令参数由不可信数据控制"},
	"Arbitrary command execution":    {ZH: "任意命令执行"},
	"Indirect command injection":     {ZH: "间接命令注入"},
	"LDAP injection":                 {ZH: "LDAP 注入"},
	"XPath injection":                {ZH: "XPath 注入"},
	"NoSQL injection":                {ZH: "NoSQL 注入"},
	"Log injection":                  {ZH: "日志注入"},
	"Template injection":             {ZH: "模板注入"},
	"Server-side template injection": {ZH: "服务端模板注入"},
	"Expression language injection":  {ZH: "表达式语言注入"},
	"Groovy injection":               {ZH: "Groovy 注入"},
	"Uncontrolled format string":     {ZH: "格式化字符串由不可信数据控制"},
	"Regular expression injection":   {ZH: "正则表达式注入"},
	"Polynomial regular expression used on uncontrolled data": {ZH: "对不可信数据使用多项式级正则表达式（ReDoS）"},
	"Regular expression denial of service":                    {ZH: "正则表达式拒绝服务（ReDoS）"},

	// ---- 路径与文件 ----
	"Uncontrolled data used in path expression": {ZH: "路径表达式使用不可信数据（路径穿越）"},
	"Path traversal": {ZH: "路径穿越"},
	"Arbitrary file write during archive extraction (Zip Slip)": {ZH: "解压归档时的任意文件写入（Zip Slip）"},
	"Arbitrary file write during tarball extraction (Tarslip)":  {ZH: "解压 tar 包时的任意文件写入（Tarslip）"},
	"Unrestricted file upload":                                  {ZH: "不受限的文件上传"},
	"Files uploaded with dangerous type":                        {ZH: "上传了危险类型的文件"},
	"Insecure temporary file":                                   {ZH: "不安全的临时文件"},
	"Overly permissive file permissions":                        {ZH: "文件权限过于宽松"},
	"Publicly writable file":                                    {ZH: "全局可写的文件"},

	// ---- 请求伪造与重定向 ----
	"Server-side request forgery":         {ZH: "服务器端请求伪造（SSRF）"},
	"Partial server-side request forgery": {ZH: "部分服务器端请求伪造（SSRF）"},
	"Request forgery":                     {ZH: "请求伪造"},
	"Cross-site request forgery":          {ZH: "跨站请求伪造（CSRF）"},
	"Missing CSRF protection":             {ZH: "缺少 CSRF 防护"},
	"Spring CSRF protection disabled":     {ZH: "Spring CSRF 防护被禁用"},
	"Open redirect":                       {ZH: "开放重定向"},
	"Client-side URL redirect":            {ZH: "客户端 URL 重定向"},
	"Unvalidated URL redirect":            {ZH: "未校验的 URL 重定向"},

	// ---- 跨站脚本 ----
	"Stored server-side cross-site scripting": {ZH: "存储型服务端跨站脚本（XSS）"},
	"Stored cross-site scripting":             {ZH: "存储型跨站脚本（XSS）"},
	"Reflected cross-site scripting":          {ZH: "反射型跨站脚本（XSS）"},
	"Client-side cross-site scripting":        {ZH: "客户端跨站脚本（XSS）"},
	"Dom-based cross-site scripting":          {ZH: "基于 DOM 的跨站脚本（XSS）"},
	"Cross-site scripting":                    {ZH: "跨站脚本（XSS）"},
	"Jinja2 autoescape disabled":              {ZH: "Jinja2 自动转义被禁用"},
	"Autoescape off":                          {ZH: "模板自动转义被关闭"},

	// ---- 信息泄露与敏感数据 ----
	"Information exposure through log files":                       {ZH: "通过日志文件泄露信息"},
	"Information exposure through an error message":                {ZH: "通过错误消息泄露信息"},
	"Clear-text logging of sensitive information":                  {ZH: "以明文记录敏感信息"},
	"Clear-text storage of sensitive information":                  {ZH: "以明文存储敏感信息"},
	"Clear-text transmission of sensitive information":             {ZH: "以明文传输敏感信息"},
	"Cleartext storage of sensitive information in a database":     {ZH: "以明文在数据库中存储敏感信息"},
	"Exposure of private information":                              {ZH: "隐私信息泄露"},
	"Privacy leak":                                                 {ZH: "隐私泄露"},
	"Use of password hash with insufficient computational effort":  {ZH: "口令哈希计算强度不足"},
	"Storing passwords in a recoverable format":                    {ZH: "以可还原格式存储口令"},
	"Sensitive information in a cookie":                            {ZH: "Cookie 中包含敏感信息"},
	"Sensitive cookie without 'HttpOnly' flag":                     {ZH: "敏感 Cookie 缺少 HttpOnly 标志"},
	"Sensitive cookie in HTTPS session without 'Secure' attribute": {ZH: "HTTPS 会话中的敏感 Cookie 缺少 Secure 属性"},

	// ---- 密码学 ----
	"Use of a broken or weak cryptographic hashing algorithm on sensitive data": {ZH: "对敏感数据使用已破解或弱哈希算法"},
	"Use of a broken or weak cryptographic algorithm":                           {ZH: "使用已破解或弱的加密算法"},
	"Weak cryptographic algorithm":                                              {ZH: "弱加密算法"},
	"Hard-coded cryptographic key":                                              {ZH: "硬编码加密密钥"},
	"Hardcoded key":                                                             {ZH: "硬编码密钥"},
	"Weak SSL/TLS protocols":                                                    {ZH: "弱 SSL/TLS 协议"},
	"Insecure TLS configuration":                                                {ZH: "不安全的 TLS 配置"},
	"Disabled certificate validation":                                           {ZH: "证书校验被禁用"},
	"Improper certificate validation":                                           {ZH: "证书校验不当"},
	"Failure to use secure cookies":                                             {ZH: "未使用安全 Cookie"},
	"Insecure randomness":                                                       {ZH: "不安全的随机数"},
	"Use of insufficiently random values":                                       {ZH: "随机值强度不足"},
	"Predictable seed in pseudo-random number generator":                        {ZH: "伪随机数生成器使用可预测的种子"},

	// ---- 反序列化 / XXE ----
	"Deserialization of untrusted data": {ZH: "反序列化不可信数据"},
	"Unsafe deserialization":            {ZH: "不安全的反序列化"},
	"XML external entity expansion":     {ZH: "XML 外部实体注入（XXE）"},
	"XML external entity":               {ZH: "XML 外部实体注入（XXE）"},

	// ---- 内存安全（C/C++/Rust）----
	"Buffer overflow":                    {ZH: "缓冲区溢出"},
	"Static buffer overflow":             {ZH: "静态缓冲区溢出"},
	"Unbounded write":                    {ZH: "无边界写入"},
	"Use after free":                     {ZH: "释放后使用（UAF）"},
	"Double free":                        {ZH: "重复释放"},
	"Integer overflow":                   {ZH: "整数溢出"},
	"Improper validation of array index": {ZH: "数组下标校验不当"},
	"Null pointer dereference":           {ZH: "空指针解引用"},
	"Return of stack address":            {ZH: "返回栈地址"},

	// ---- 调试与配置 ----
	"Django app is run in debug mode":            {ZH: "Django 应用以调试模式运行"},
	"Active debug code":                          {ZH: "存在启用的调试代码"},
	"Binding a socket to all network interfaces": {ZH: "套接字绑定到所有网络接口"},
	"Insecure default":                           {ZH: "不安全的默认配置"},
	"Missing rate limiting":                      {ZH: "缺少速率限制"},
	"Resource exhaustion":                        {ZH: "资源耗尽"},
	"Denial of service":                          {ZH: "拒绝服务"},
	"Improper access control":                    {ZH: "访问控制不当"},
	"Missing authentication":                     {ZH: "缺少身份认证"},
	"Authorization bypass":                       {ZH: "认证绕过"},

	// ---- 其他常见 ----
	"Prototype pollution":                     {ZH: "原型链污染"},
	"Prototype pollution in utility function": {ZH: "工具函数中的原型链污染"},
	"Unsafe dynamic method access":            {ZH: "不安全的动态方法访问"},
	"Unvalidated dynamic method call":         {ZH: "未校验的动态方法调用"},
	"Unsafe reflection":                       {ZH: "不安全的反射调用"},
	"Insecure download of executable":         {ZH: "不安全地下载可执行文件"},
	"Non-constant time comparison":            {ZH: "非恒定时间比较（时序攻击）"},
	"Race condition":                          {ZH: "竞态条件"},
	"Symlink attack":                          {ZH: "符号链接攻击"},
	"Missing encryption of sensitive data":    {ZH: "敏感数据未加密"},
	"Unreachable code":                        {ZH: "不可达代码"},
}

// builtinLabels 是证据链标签的词典。
var builtinLabels = map[string]string{
	"sink":                    "污点汇聚点",
	"Sink":                    "污点汇聚点",
	"source":                  "污点来源",
	"Source":                  "污点来源",
	"user-provided value":     "用户可控输入",
	"Stack trace information": "堆栈跟踪信息",
	"used as credentials":     "被用作凭据",
	"hardcoded value":         "硬编码值",
	"Hardcoded value":         "硬编码值",
	"untrusted data":          "不可信数据",
	"this query":              "该查询",
	"remote input":            "远端输入",
	"database query":          "数据库查询",
	"command line":            "命令行参数",
	"file path":               "文件路径",
	"URL":                     "URL",
	"sensitive data":          "敏感数据",
	"credential":              "凭据",
}

// builtinLabelTemplates 是带动态部分的标签模板，键使用 {{W}} 作为占位符。
var builtinLabelTemplates = map[string]string{
	"ControlFlowNode for {{W}}": "{{W}} 的控制流节点",
	"ControlFlowNode":           "控制流节点",
	"DataFlowNode for {{W}}":    "{{W}} 的数据流节点",
	"SSA variable {{W}}":        "SSA 变量 {{W}}",
	"hardcoded {{W}}":           "硬编码的 {{W}}",
}

// builtinTemplates 是消息模板词典：键为规范化后的英文，值为带同名占位符的中文模板。
//
// 占位符：
//
//	{{API}}  API/函数名（保持原样）
//	{{N}}    数字参数（position 下标等）
//	{{W}}    标识符（keyword 名等）
//	{{LINK}} 证据链位置引用
var builtinTemplates = map[string]string{
	"Call to {{API}} [position {{N}}] with untrusted data from {{LINK}}.": "调用 {{API}}（第 {{N}} 个参数）时，传入了来自 {{LINK}} 的不可信数据。",
	"Call to {{API}} [keyword {{W}}] with untrusted data from {{LINK}}.":  "调用 {{API}}（关键字参数 {{W}}）时，传入了来自 {{LINK}} 的不可信数据。",
	"Call to {{API}} with untrusted data from {{LINK}}.":                  "调用 {{API}} 时，传入了来自 {{LINK}} 的不可信数据。",
	"Call to {{API}} [position {{N}}] with untrusted data.":               "调用 {{API}}（第 {{N}} 个参数）时传入了不可信数据。",
	"Call to {{API}} with untrusted data.":                                "调用 {{API}} 时传入了不可信数据。",

	"{{LINK}} flows to this location and may be exposed to an external user.": "{{LINK}} 会流动到此处，可能被暴露给外部用户。",
	"The full URL of this request depends on a {{LINK}}.":                     "该请求的完整 URL 取决于 {{LINK}}。",
	"This SQL query depends on a {{LINK}}.":                                   "该 SQL 查询取决于 {{LINK}}。",
	"This database query depends on a {{LINK}}.":                              "该数据库查询取决于 {{LINK}}。",
	"Cross-site scripting vulnerability due to a {{LINK}}.":                   "由于 {{LINK}}，存在跨站脚本（XSS）漏洞。",
	"This hardcoded value is {{LINK}}.":                                       "此硬编码值来自 {{LINK}}。",
	"This value is {{LINK}}.":                                                 "此值来自 {{LINK}}。",
	"The value of this {{W}} depends on a {{LINK}}.":                          "此 {{W}} 的取值取决于 {{LINK}}。",
	"A Flask app appears to be run in debug mode. This may allow an attacker to run arbitrary code through the debugger.": "检测到 Flask 应用以调试模式运行，攻击者可能通过调试器执行任意代码。",
	"This query depends on a {{LINK}}.":             "该查询取决于 {{LINK}}。",
	"Sensitive data from {{LINK}} is exposed here.": "来自 {{LINK}} 的敏感数据在此处被暴露。",
	"{{LINK}} is used here without sanitization.":   "{{LINK}} 未经净化就被使用。",
}
