from {{package_name}}.core import get_status


def test_get_status():
    status = get_status()
    assert status["package"] == "{{repo_name}}"
    assert status["status"] == "operational"
