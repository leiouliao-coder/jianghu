extends Control
## Development entry page: HTTP health check and a link to the account screen.

signal health_checked(ok: bool, message: String)

@onready var server_address: LineEdit = $Margin/Content/ServerAddress
@onready var check_button: Button = $Margin/Content/CheckHealth
@onready var status_label: Label = $Margin/Content/Status
@onready var health_request: HTTPRequest = $HealthRequest
@onready var account_button: Button = $Margin/Content/OpenAccount


func _ready() -> void:
	check_button.pressed.connect(check_health)
	account_button.pressed.connect(open_account)
	health_request.request_completed.connect(_on_request_completed)


func check_health() -> void:
	if check_button.disabled:
		return
	var base_url: String = server_address.text.strip_edges().trim_suffix("/")
	if not (base_url.begins_with("http://") or base_url.begins_with("https://")):
		_finish(false, "地址必须以 http:// 或 https:// 开头。")
		return
	check_button.disabled = true
	status_label.text = "正在检查 / Checking…"
	var request_error: Error = health_request.request(base_url + "/healthz")
	if request_error != OK:
		_finish(false, "无法发起请求：%s" % error_string(request_error))


func _on_request_completed(
	result: int, response_code: int, _headers: PackedStringArray, body: PackedByteArray
) -> void:
	if result != HTTPRequest.RESULT_SUCCESS:
		_finish(false, "连接失败（网络结果 %d）；请确认服务已启动及地址可达。" % result)
		return
	if response_code != 200:
		_finish(false, "服务返回 HTTP %d。" % response_code)
		return
	var parser: JSON = JSON.new()
	if parser.parse(body.get_string_from_utf8()) != OK:
		_finish(false, "响应不是有效 JSON。")
		return
	var payload: Variant = parser.data
	if typeof(payload) != TYPE_DICTIONARY:
		_finish(false, "健康检查响应不是 JSON 对象。")
		return
	if payload.get("status", "") != "ok":
		_finish(false, "服务状态不是 ok。")
		return
	_finish(true, "服务正常 / Server healthy (HTTP 200, status=ok)")


func _finish(ok: bool, message: String) -> void:
	check_button.disabled = false
	status_label.text = message
	health_checked.emit(ok, message)


func open_account() -> void:
	get_tree().change_scene_to_file("res://scenes/account.tscn")
