extends SceneTree
## No network/server dependency. Run with --headless --path client --script ...

var failures: int = 0


func _initialize() -> void:
	call_deferred("_run")


func _check(condition: bool, message: String) -> void:
	if not condition:
		failures += 1
		push_error(message)


func _run() -> void:
	var scene: PackedScene = load("res://scenes/main.tscn") as PackedScene
	_check(scene != null, "Main scene must load")
	if scene == null:
		quit(1)
		return
	var main: Control = scene.instantiate() as Control
	root.add_child(main)
	await process_frame
	_check(main != null, "Main scene must instantiate")
	var button: Button = main.get_node("Margin/Content/CheckHealth") as Button
	var status: Label = main.get_node("Margin/Content/Status") as Label
	var address: LineEdit = main.get_node("Margin/Content/ServerAddress") as LineEdit
	_check(button.pressed.is_connected(Callable(main, "check_health")), "Button wired to handler")
	_check(address.text == "http://127.0.0.1:8080", "Default development address")
	address.text = "invalid-url"
	button.pressed.emit()
	_check(not button.disabled, "Invalid URL must not leave UI locked")
	_check(status.text.contains("http://"), "Invalid URL displays actionable error")
	main.call("_on_request_completed", HTTPRequest.RESULT_SUCCESS, 200, PackedStringArray(), '{"status":"ok"}'.to_utf8_buffer())
	_check(status.text.contains("Server healthy"), "Valid payload accepted")
	main.call("_on_request_completed", HTTPRequest.RESULT_SUCCESS, 200, PackedStringArray(), '[]'.to_utf8_buffer())
	_check(not status.text.contains("Server healthy"), "Non-object JSON rejected")
	main.call("_on_request_completed", HTTPRequest.RESULT_SUCCESS, 200, PackedStringArray(), 'bad-json'.to_utf8_buffer())
	_check(status.text.contains("JSON"), "Malformed JSON rejected")
	main.call("_on_request_completed", HTTPRequest.RESULT_SUCCESS, 503, PackedStringArray(), '{}'.to_utf8_buffer())
	_check(status.text.contains("503"), "Non-200 response rejected")
	main.call("_on_request_completed", HTTPRequest.RESULT_CANT_CONNECT, 0, PackedStringArray(), PackedByteArray())
	_check(status.text.contains("连接失败"), "Network failure explained")
	main.queue_free()
	await process_frame
	if failures == 0:
		print("PASS bootstrap scene and health response checks")
	quit(0 if failures == 0 else 1)
