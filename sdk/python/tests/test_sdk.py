from motionmesh import MotionMeshClient

def test_client_init():
    client = MotionMeshClient("mot_live_test")
    assert client.get_api_key() == "mot_live_test"
