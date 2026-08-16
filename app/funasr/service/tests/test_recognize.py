"""批量识别文本清理测试。"""
from funasr_server.recognize import clean_text


def test_clean_text_removes_sensevoice_tags():
    text = "<|zh|><|NEUTRAL|><|Speech|><|woitn|>来公布一下昨天的情况"
    assert clean_text(text) == "来公布一下昨天的情况"


def test_clean_text_normalizes_whitespace():
    text = "好，  呃，昨天的   情况"
    assert clean_text(text) == "好， 呃，昨天的 情况"


def test_clean_text_empty():
    assert clean_text("") == ""
