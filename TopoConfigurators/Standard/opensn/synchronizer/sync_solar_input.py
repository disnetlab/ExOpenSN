from etcd3 import Etcd3Client   # lets Python communicate with etcd
from opensn.const.etcd_key import SOLAR_INPUT_LIST_KEY
from opensn.model.solar_input import SolarInput, solar_input_from_json, solar_input_to_json

# writes one satellite's SolarInput to etcd
def put_solar_input(etcd_client: Etcd3Client, instance_id: str, solar_input: SolarInput):
    solar_input_key = "%s/%s" % (SOLAR_INPUT_LIST_KEY, instance_id)
    etcd_client.put(solar_input_key, solar_input_to_json(solar_input))


def get_solar_input(etcd_client: Etcd3Client, instance_id) -> SolarInput:
    solar_input_key = "%s/%s" % (SOLAR_INPUT_LIST_KEY, instance_id)
    val, meta = etcd_client.get(solar_input_key)
    return solar_input_from_json(val)


def get_solar_input_map(etcd_client: Etcd3Client) -> dict[str, SolarInput]:
    ret: dict[str, SolarInput] = {}
    resps = etcd_client.get_prefix(SOLAR_INPUT_LIST_KEY)
    for val, meta in resps:
        # ret[extract the instance ID from the etcd key] = SolarInput obj
        ret[meta.key.decode().split('/')[-1]] = solar_input_from_json(val)
    return ret
