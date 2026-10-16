extends SceneTree
## Real integration check against a running server; default localhost:8080.

var finished: bool = false


func _initialize() -> void:
	call_deferred("_run")


func _run() -> void:
	var scene: PackedScene = load("res://scenes/main.tscn") as PackedScene
	if scene == null:
		push_error("Main scene failed to load")
		quit(1)
		return
	var main: Control = scene.instantiate() as Control
	root.add_child(main)
	await process_frame
	var address: LineEdit = main.get_node("Margin/Content/ServerAddress") as LineEdit
	var arguments: PackedStringArray = OS.get_cmdline_user_args()
	if not arguments.is_empty():
		address.text = arguments[0]
	main.connect("health_checked", _on_health_checked)
	main.call("check_health")
	await create_timer(8.0).timeout
	if not finished:
		push_error("Health check did not finish before deadline")
		quit(1)


func _on_health_checked(ok: bool, message: String) -> void:
	finished = true
	print("PASS " + message if ok else "FAIL " + message)
	quit(0 if ok else 1)
