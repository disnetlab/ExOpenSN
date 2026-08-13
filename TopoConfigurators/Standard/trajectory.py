
import math
import ephem
import datetime
from opensn.const.const_var import R_EARTH,LIGHT_SPEED_M_S
from opensn.model.position import Position
from instance_types import TYPE_SATELLITE,TYPE_GROUND_STATION
from instance_types import EX_TLE0_KEY,EX_TLE1_KEY,EX_TLE2_KEY,EX_LATITUDE_KEY,EX_LONGITUDE_KEY,EX_ALTITUDE_KEY
from opensn.model.instance import Instance

def deg2rad(deg: float) -> float:
    return deg / 180 * math.pi

def calculate_postion(instance: Instance,time:datetime.datetime) -> Position:
    ret = Position()
    if instance.type == TYPE_SATELLITE and instance.start:
        ephem_time = ephem.Date(time)
        ephem_obj = ephem.readtle(
            instance.extra[EX_TLE0_KEY],
            instance.extra[EX_TLE1_KEY],
            instance.extra[EX_TLE2_KEY],
        )
        ephem_obj.compute(ephem_time)
        ret.latitude = ephem_obj.sublat
        ret.longitude = ephem_obj.sublong
        ret.altitude = ephem_obj.elevation
    elif instance.type == TYPE_GROUND_STATION:
        ret.latitude = deg2rad(float(instance.extra[EX_LATITUDE_KEY]))
        ret.longitude = deg2rad(float(instance.extra[EX_LONGITUDE_KEY]))
        ret.altitude = deg2rad(float(instance.extra[EX_ALTITUDE_KEY]))
    return ret

def distance_meter(one:Position,another:Position) -> float: # meter
    z1 = (one.altitude+R_EARTH) * math.sin(one.latitude)
    base1 = (one.altitude+R_EARTH) * math.cos(one.latitude)
    x1 = base1 * math.cos(one.longitude)
    y1 = base1 * math.sin(one.longitude)
    z2 = (another.altitude+R_EARTH) * math.sin(another.latitude)
    base2 = (another.altitude+R_EARTH) * math.cos(another.latitude)
    x2 = base2 * math.cos(another.longitude)
    y2 = base2 * math.sin(another.longitude)
    return math.sqrt((x1-x2)**2+(y1-y2)**2+(z1-z2)**2)

def get_propagation_delay_s(distance_meter:float) -> float: # second
    return distance_meter / LIGHT_SPEED_M_S

def select_closest_satellite(
        ground_station:Instance,
        position_map:dict[str,Position],
        instance_map:dict[str,Instance]
    ) -> (str,bool) :
    closet_distance = math.inf
    select_satellite_id = ""
    change = True
    for instance_id,instance_info in instance_map.items():
        if instance_info.type != TYPE_SATELLITE:
            continue
        new_distance = distance_meter(
            position_map[instance_id],
            position_map[ground_station.instance_id],
        )
        if new_distance < closet_distance:
            closet_distance = new_distance
            select_satellite_id = instance_id
    if len(ground_station.connections) < 0 and select_satellite_id == "":
        return "",False
    
    for end_info in ground_station.connections.values():
        if select_satellite_id == end_info.instance_id:
            change = False
    return select_satellite_id,change

""" MY EXTENSION:
Use the satellite's existing TLE and the simulation time with PyEphem to determine
the Sun's altitude from the satellite's location, then convert that altitude into gamma """
def calculate_sun_angle(instance_info, ephem_time) -> float:
    """
    Returns gamma: the angle (radians) between the satellite's solar panel
    normal and the sun vector, at the given time.

    0         --> sun directly overhead the panel (max generation);
    pi/2  <=  --> sun below the panel plane /
                  no generation (satellite in eclipse or panel edge-on to the sun).
 
    Simplification: assumes the panel is always sun-pointing on the axis
    that matters (i.e. this returns the angle between the satellite's
    position vector and the sun vector, treating the panel as nadir/zenith
    oriented). If your satellite model has independent panel articulation,
    this needs a more detailed attitude model. This is a standard simplification for
    constellation-level power estimates, not real single-satellite ADCS
    (attitude determination and control) modeling.
    """
    # Reuse the same TLE-based ephem object construction already used in
    # calculate_postion() for this instance, so gamma is computed from the
    # exact same orbital state as position — no risk of the two drifting
    # out of sync from being computed via different means.
    
    ephem_obj = ephem.readtle(
        instance_info.extra[EX_TLE0_KEY],
        instance_info.extra[EX_TLE1_KEY],
        instance_info.extra[EX_TLE2_KEY],
    )

    # Calculate satellite position at that time
    ephem_obj.compute(ephem_time)

    # Create the Sun 
    sun = ephem.Sun()

    # Sun's astronomical position at the simulation time.
    sun.compute(ephem_time)
 
    # ephem gives sub-observer-style angles for a body's position; for a
    # simple sun-pointing-panel approximation, the angle between the
    # satellite's sub-point-to-sun direction and its local zenith is a
    # reasonable, standard estimate. `ephem` doesn't give this directly for
    # two arbitrary bodies without an observer, so compute it via the
    # satellite's own position as the observer:

    # Create an observer at the satellite's location
    observer = ephem.Observer() 
    observer.lat = str(ephem_obj.sublat)       #latitude
    observer.long = str(ephem_obj.sublong)     # longitude
    observer.elevation = max(ephem_obj.elevation, 0)   # set observer's altitude
    observer.date = ephem_time
    sun.compute(observer)
 
    # sun.alt is the sun's altitude above the local horizon at the
    # satellite's sub-point, as seen by an observer there.
    # Panel-normal angle gamma = 90 degrees - altitude (sun at zenith -> gamma = 0).
    gamma = (math.pi / 2) - float(sun.alt)
 
    # Eclipse / below-horizon: clamp so callers don't need to special-case
    # negative-altitude sun positions separately from the physical "no
    # generation" case already handled in FullPowerModel.Generate (Go side
    # clamps on cos(gamma) <= 0, so any gamma >= pi/2 here has the same
    # zero-generation effect regardless of the exact value beyond that).
    return max(0.0, gamma)
