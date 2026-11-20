import argparse
import collections
import xml.etree.ElementTree as xml


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("dump", help="真实 Android 会话页的 UIAutomator XML")
    parser.add_argument("user_message")
    parser.add_argument("model_reply")
    args = parser.parse_args()
    nodes = [node for node in xml.parse(args.dump).iter("node") if node.get("package") == "com.amitia.amitia_app"]
    counts = collections.Counter(
        (node.get("text") or node.get("content-desc") or "").strip()
        for node in nodes
        if node.get("class") != "android.widget.EditText"
        and node.get("long-clickable") == "true"
    )
    replies = [node for node in nodes if (node.get("text") or node.get("content-desc") or "").strip() == args.model_reply]
    print(f"用户消息正文={counts[args.user_message]}，模型回复={len(replies)}", flush=True)
    assert counts[args.user_message] == 1, counts
    assert len(replies) == 1, [node.attrib for node in replies]


if __name__ == "__main__":
    main()
