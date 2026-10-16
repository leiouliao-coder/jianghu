extends SceneTree
## Manual visual check, not run in CI (needs a rendering window): logs in
## through the real account shell, enters the world, walks, and saves
## screenshots. Usage, with a server that has a database:
##   godot --path client --resolution 1280x800 --script res://tests/capture_world.gd -- <out_dir> [ws://127.0.0.1:8080/v1/ws]
var out_dir: String = ""
func _initialize() -> void:
	call_deferred("_run")
func _wait(sec: float) -> void:
	await create_timer(sec).timeout
func _run() -> void:
	var args: PackedStringArray = OS.get_cmdline_user_args()
	out_dir = args[0]
	var url: String = args[1] if args.size() > 1 else "ws://127.0.0.1:8080/v1/ws"
	var shell: Control = (load("res://scenes/account.tscn") as PackedScene).instantiate()
	root.add_child(shell)
	await process_frame
	shell.get_node("Margin/Content/Address").text = url
	var name: String = "shot%d" % (Time.get_unix_time_from_system())
	shell.get_node("Margin/Content/Name").text = name
	shell.get_node("Margin/Content/Password").text = "password123"
	shell.call("_on_register"); await _wait(1.0)
	shell.call("_on_login"); await _wait(1.0)
	shell.get_node("Margin/Content/Characters/NewName").text = "剑客" + name.right(3)
	shell.call("_on_create"); await _wait(1.0)
	get_root().get_texture().get_image().save_png(out_dir + "/1_characters.png")
	shell.get_node("Margin/Content/Characters/List").select(0)
	shell.call("_on_enter"); await _wait(1.5)
	get_root().get_texture().get_image().save_png(out_dir + "/2_world_A.png")
	shell.world_view.request_move(Vector2(-40, 0)); await _wait(3.0)
	get_root().get_texture().get_image().save_png(out_dir + "/3_walking_west.png")
	shell.world_view.rig.zoom(10.0); shell.world_view.rig.rotate_by(-150.0); await _wait(0.5)
	get_root().get_texture().get_image().save_png(out_dir + "/4_zoomed_out.png")
	shell.world_view.request_move(Vector2(200, -130)); await _wait(0.8)
	get_root().get_texture().get_image().save_png(out_dir + "/5_risk_prompt.png")
	quit(0)
