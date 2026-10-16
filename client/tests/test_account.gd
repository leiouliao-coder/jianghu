extends SceneTree
## Account screen wiring and reply handling; no network.

var failures: int = 0


func _initialize() -> void:
	call_deferred("_run")


func _check(condition: bool, message: String) -> void:
	if not condition:
		failures += 1
		push_error(message)


func _reply(screen: Control, req_type: String, envelope: Dictionary) -> void:
	var req_id: String = "t-%d" % randi()
	screen._pending[req_id] = req_type
	screen._on_reply(req_id, envelope)


func _run() -> void:
	var scene: PackedScene = load("res://scenes/account.tscn") as PackedScene
	_check(scene != null, "account scene loads")
	var screen: Control = scene.instantiate() as Control
	root.add_child(screen)
	await process_frame
	var status: Label = screen.get_node("Margin/Content/Status") as Label
	var chars: VBoxContainer = screen.get_node("Margin/Content/Characters") as VBoxContainer
	var list: ItemList = screen.get_node("Margin/Content/Characters/List") as ItemList
	_check(not chars.visible, "character panel hidden before login")

	_check(screen.validate_credentials("x", "password1") != "", "short name rejected")
	_check(screen.validate_credentials("顾砚", "short") != "", "short password rejected")
	_check(screen.validate_credentials("顾砚", "password1") == "", "valid credentials accepted")

	(screen.get_node("Margin/Content/Name") as LineEdit).text = "a"
	(screen.get_node("Margin/Content/AuthButtons/Login") as Button).pressed.emit()
	_check(status.text.contains("名字"), "invalid input explained without connecting")

	(screen.get_node("Margin/Content/Name") as LineEdit).text = "顾砚"
	(screen.get_node("Margin/Content/Password") as LineEdit).text = "password1"
	(screen.get_node("Margin/Content/Address") as LineEdit).text = "http://wrong"
	(screen.get_node("Margin/Content/AuthButtons/Login") as Button).pressed.emit()
	_check(status.text.contains("ws://"), "non-websocket address explained")

	_reply(screen, "login", {"type": "login.ok", "payload": {"account_id": 1, "token": "tok"}})
	_check(screen.token == "tok" and chars.visible, "login stores token in memory and shows characters")
	_reply(screen, "char_list", {"type": "char_list.ok", "payload": {
		"characters": [{"id": 5, "name": "剑客"}, {"id": 6, "name": "拳师"}],
		"present": {"character_id": 6, "state": "present"}}})
	_check(list.item_count == 2 and list.get_item_text(1).contains("在江湖中"), "list marks present character")
	_reply(screen, "enter_world", {"type": "error", "payload": {"code": "character_present", "detail": {"character_id": 6}}})
	_check(status.text.contains("另一名角色"), "character_present explained")
	_reply(screen, "char_create", {"type": "error", "payload": {"code": "something_new"}})
	_check(status.text.contains("something_new"), "unknown error code still shown")
	_reply(screen, "resume", {"type": "error", "payload": {"code": "bad_credentials"}})
	_check(screen.token == "" and not chars.visible, "failed resume clears session")

	screen.call("_on_closed", Protocol.CLOSE_KICKED, "")
	_check(status.text.contains("其他设备"), "kick explained")

	screen.queue_free()
	await process_frame
	if failures == 0:
		print("PASS account screen wiring and reply handling")
	quit(0 if failures == 0 else 1)
