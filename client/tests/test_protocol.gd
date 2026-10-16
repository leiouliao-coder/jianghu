extends SceneTree
## Protocol v1 envelope and reconnect rules; no network.

var failures: int = 0


func _initialize() -> void:
	call_deferred("_run")


func _check(condition: bool, message: String) -> void:
	if not condition:
		failures += 1
		push_error(message)


func _run() -> void:
	var text: String = Protocol.encode("login", "r-1", {"name": "顾砚"})
	var parsed: Dictionary = JSON.parse_string(text)
	_check(parsed["v"] == 1.0 and parsed["type"] == "login" and parsed["req_id"] == "r-1", "encode envelope fields")
	_check(parsed["payload"]["name"] == "顾砚", "encode keeps UTF-8 payload")
	_check(not JSON.parse_string(Protocol.encode("ping", "r-2")).has("payload"), "null payload omitted")

	var ok: Dictionary = Protocol.decode('{"v":1,"type":"login.ok","req_id":"r-1","seq":3,"server_time":1,"payload":{"account_id":7}}')
	_check(ok.get("type") == "login.ok" and ok.get("seq") == 3, "decode valid reply")
	_check(ok["payload"]["account_id"] == 7.0, "decode payload")
	var push: Dictionary = Protocol.decode('{"v":1,"type":"kicked","seq":1}')
	_check(push.get("payload") == {}, "missing payload becomes empty dictionary")
	for bad: String in ["not json", "[]", '{"v":2,"type":"x","seq":1}', '{"v":1,"seq":1}', '{"v":1,"type":"","seq":1}', '{"v":1,"type":"x"}', '{"v":1,"type":"x","seq":0}', '{"v":1,"type":"x","seq":1,"payload":[1]}']:
		_check(Protocol.decode(bad).is_empty(), "reject invalid server message: " + bad)

	_check(is_equal_approx(Protocol.backoff_seconds(0), 0.5), "first backoff 0.5s")
	_check(is_equal_approx(Protocol.backoff_seconds(3), 4.0), "backoff doubles")
	_check(is_equal_approx(Protocol.backoff_seconds(20), 30.0), "backoff capped at 30s")
	_check(Protocol.is_final_close(4001) and Protocol.is_final_close(4002) and Protocol.is_final_close(4003), "final close codes")
	_check(not Protocol.is_final_close(1001) and not Protocol.is_final_close(1006), "recoverable close codes")

	var client: WsClient = WsClient.new()
	root.add_child(client)
	_check(client.connect_to("http://127.0.0.1:8080") == ERR_INVALID_PARAMETER, "reject non-ws URL")
	_check(client.request("ping") == "", "no request while disconnected")
	client.queue_free()
	await process_frame
	if failures == 0:
		print("PASS protocol envelope, validation and backoff checks")
	quit(0 if failures == 0 else 1)
