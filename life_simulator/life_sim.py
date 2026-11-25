#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""人生模拟器：一次从出生到终老的推演。

用法：
    python3 life_sim.py
    python3 life_sim.py --seed 7 --name 林越
"""

from __future__ import annotations

import argparse
import random
from dataclasses import dataclass, field
from typing import Callable


def clamp(value: float, low: int = 0, high: int = 100) -> int:
    return int(max(low, min(high, value)))


@dataclass
class Person:
    name: str
    gender: str
    age: int = 0
    health: int = 72
    intelligence: int = 52
    physique: int = 58
    charm: int = 50
    happiness: int = 62
    wealth: int = 35
    height: int = 188
    pingpong: int = 95
    math: int = 45
    music: int = 38
    flags: set[str] = field(default_factory=set)
    last_event: dict[str, int] = field(default_factory=dict)
    tries: dict[str, int] = field(default_factory=dict)
    story: list[tuple[int, str]] = field(default_factory=list)
    alive: bool = True
    cause: str = ""

    def say(self, text: str) -> None:
        self.story.append((self.age, text))

    def shift(self, **deltas: int) -> None:
        for key, delta in deltas.items():
            if key == "height":
                self.height += delta
                continue
            setattr(self, key, clamp(getattr(self, key) + delta))

    def has(self, flag: str) -> bool:
        return flag in self.flags

    def mark(self, flag: str) -> None:
        self.flags.add(flag)


@dataclass
class Event:
    key: str
    start: int
    end: int
    weight: Callable[[Person], float]
    run: Callable[[Person], str | None]
    repeatable: bool = False
    cooldown: int = 0


def ev_fever(p: Person) -> str:
    p.shift(health=-4, intelligence=-1)
    return "半夜发起高烧，父母抱着你跑了两家医院，输液输到天亮。"


def ev_words(p: Person) -> str:
    p.shift(intelligence=3, charm=1)
    return "十个月就会叫爸爸妈妈，邻居说这孩子嘴甜，将来不吃亏。"


def ev_fall(p: Person) -> str:
    p.shift(physique=1, happiness=-2)
    return "学走路时磕在茶几角上，眉骨留了道小疤，哭得整栋楼都听见。"


def ev_kindergarten(p: Person) -> str:
    p.shift(charm=3, happiness=2)
    return "幼儿园老师最喜欢你，因为你搭的积木总比别的小朋友高一层。"


def ev_grandpa_ball(p: Person) -> str:
    p.shift(pingpong=2, happiness=2)
    p.mark("touched_racket")
    return "爷爷用木板和旧海绵做了只球拍，在饭桌上教你颠球，你一次颠了三十七个。"


def ev_coach_find(p: Person) -> str:
    p.mark("sports_school")
    p.shift(physique=4, happiness=3)
    return (
        "体育老师在课间把你留下，让你接他抽过来的十个球。你接住九个，"
        "他沉默了一会儿说：明天跟我去体校。"
    )


def ev_growth(p: Person) -> str:
    p.height = min(198, p.height + 3)
    p.shift(physique=2, health=-1)
    return "一个暑假长了八厘米，校服裤子短了一截，走路总像踩在别人的鞋上。"


def ev_glasses(p: Person) -> str:
    p.shift(health=-2, charm=-2)
    return "查视力查出近视，配了人生第一副眼镜，被同学叫了半个月四眼。"


def ev_math(p: Person) -> str:
    p.shift(intelligence=4, happiness=2)
    return "数学竞赛拿了区里二等奖，奖品是一套圆规，你在文具盒里放了三年。"


def ev_bully(p: Person) -> str:
    p.shift(happiness=-5, charm=1)
    return "高年级的学生堵在楼道要零花钱，你没给，挨了一拳，也第一次学会还手。"


def ev_first_match(p: Person) -> str:
    p.shift(physique=3, happiness=3)
    return "第一次代表体校出去打比赛，小组赛三胜一负，输的那场你记了很多年。"


def ev_provincial(p: Person) -> str:
    p.mark("provincial")
    p.shift(physique=5, happiness=5, wealth=3)
    return "省队来挑人，教练只说了两个字：要了。你收拾行李那天，母亲在厨房站了很久。"


def ev_basketball(p: Person) -> str:
    p.shift(physique=2, happiness=1)
    p.mark("basketball_offer")
    return (
        "篮球教练在操场拦住你，说这个身高打乒乓球是浪费，来篮下站着都比别人高一头。"
        "你想了两天，还是回了球馆。"
    )


def ev_injury(p: Person) -> str:
    p.shift(health=-8, physique=-3, happiness=-6)
    p.mark("injury")
    return "一次侧身暴冲之后，膝盖里像被谁拧了一下，队医按了两下就让你停训。"


def ev_crush(p: Person) -> str:
    p.shift(happiness=4, charm=2)
    return "同桌把橡皮切成两半，一半给你。你把这件小事记了整整一个学期。"


def ev_rebel(p: Person) -> str:
    p.shift(happiness=-4, intelligence=1)
    return "和父亲大吵一架，摔门出去在楼下坐到半夜，回家时桌上留着一碗没凉透的面。"


def ev_national(p: Person) -> str:
    p.mark("national")
    p.shift(physique=6, happiness=8, wealth=6)
    return "国家队集训名单下来，你的名字在最后一页。走进训练馆那天，墙上挂着三代世界冠军的照片。"


def ev_first_love(p: Person) -> str:
    p.shift(happiness=6, charm=3)
    p.mark("first_love")
    return "在队里的康复室认识了一个练跳水的姑娘，你们隔着冰袋聊了一整个下午。"


def ev_sponsor(p: Person) -> str:
    p.shift(wealth=8, charm=2)
    return "第一份球拍代言找上门，签字那天你才发现，自己的名字原来可以印在东西上。"


def ev_university(p: Person) -> str:
    p.shift(intelligence=6, happiness=4)
    p.mark("university")
    return "你把录取通知书摊在球台上看了很久，最后决定去读书，球拍收进了柜子最上层。"


def ev_worlds(p: Person) -> str:
    n = p.tries.get("worlds", 0)
    roll = p.pingpong + p.physique // 3 + random.randint(-25, 25)
    if roll >= 115:
        p.mark("world_champion")
        p.shift(wealth=25, happiness=15, charm=10)
        return "世乒赛男单决赛，你在决胜局 10 平后连拿两分，把球拍扔向了看台。"
    if roll >= 95:
        p.shift(wealth=12, happiness=8)
        finals = [
            "世乒赛打到四强，半决赛七局惜败，领奖台上你笑得比冠军还用力。",
            "世乒赛又一次停在半决赛，混采区有人问你还能打几年，你说不知道。",
            "世乒赛拿了一块铜牌，回程飞机上你反复看那场球的录像，看到空乘来关灯。",
        ]
        return finals[n % len(finals)]
    p.shift(happiness=-6, wealth=3)
    early = [
        "世乒赛止步十六强，回酒店的路上谁都没说话，教练只拍了拍你的肩。",
        "世乒赛第一轮就遇上队友，你输了，坐在看台上把那场球从头看到尾。",
        "世乒赛资格赛打了三场，正赛第一天就收拾球包回了房间。",
    ]
    return early[n % len(early)]


def ev_olympics(p: Person) -> str:
    roll = p.pingpong + p.physique // 3 + random.randint(-30, 30)
    if roll >= 120:
        p.mark("olympic_gold")
        p.shift(wealth=35, happiness=20, charm=15)
        return "奥运会决赛，你在全场嘘声里赢下最后一分，国歌响起时你哭得说不出话。"
    if roll >= 100:
        p.shift(wealth=18, happiness=10)
        return "奥运会拿到一枚铜牌，你在混合采访区说，四年后再来。"
    p.shift(happiness=-8)
    return "奥运会首轮抽到种子选手，五局之后你走下场，观众席上有面旗子还在挥。"


def ev_coach(p: Person) -> str:
    p.mark("coach")
    p.shift(wealth=6, happiness=6, charm=3)
    return "省队请你回去当教练，你带着一群十几岁的孩子，第一堂课只教了握拍。"


def ev_marriage(p: Person) -> str:
    p.mark("married")
    p.shift(happiness=8, wealth=3)
    return "婚礼办得不大，来的多是球馆里的人。敬酒时有人喊，以后孩子归谁练。"


def ev_child(p: Person) -> str:
    p.mark("child")
    p.shift(happiness=8, health=-2, wealth=-5)
    return "孩子出生那天，你在产房外站了六个小时，护士抱出来时你不敢伸手。"


def ev_parents_ill(p: Person) -> str:
    p.shift(happiness=-8, health=-2)
    p.mark("parents_ill")
    return "父亲住院，你在走廊的塑料椅上守了三夜，第一次发现他手上的老年斑那么多。"


def ev_knee(p: Person) -> str:
    p.shift(health=-6, happiness=-3)
    return "旧伤在阴雨天准时发作，你学会了看天气预报决定第二天走多少路。"


def ev_invest(p: Person) -> str:
    gain = random.randint(-12, 20)
    p.shift(wealth=gain, happiness=3 if gain > 0 else -4)
    if gain > 0:
        return "朋友拉你合伙开了家球馆，第三年回本，账上第一次有了闲钱。"
    return "听人劝投了个项目，两年后只剩一张收据，你把这件事讲给每个想发财的人听。"


def ev_retire_life(p: Person) -> str:
    p.mark("retired_life")
    p.shift(happiness=4)
    return "办了退休手续，早上终于可以不去球馆，你却六点就醒了，坐在阳台上等天亮。"


def ev_grandchild(p: Person) -> str:
    p.mark("grandchild")
    p.shift(happiness=8, health=-1)
    return "孙子第一次来家里，你翻出那只旧球拍，发现海绵已经硬得像块木头。"


def ev_chronic(p: Person) -> str:
    p.shift(health=-8)
    return "体检查出血压高，医生让你戒烟少盐，你点头，回家照旧吃了一碗咸菜面。"


def ev_eye(p: Person) -> str:
    p.shift(health=-4, happiness=-2)
    return "白内障手术排在周三上午，进手术室前你把眼镜摘下来，忽然觉得走廊特别长。"


def ev_hearing(p: Person) -> str:
    p.shift(health=-3, happiness=-3, charm=-2)
    return "助听器配回来的第一天，你嫌屋里太吵，第二天就再没摘下来过。"


def ev_team_grow(p: Person) -> str:
    p.shift(happiness=7, wealth=3)
    return "你带的队员打进省队，出发那天他在车站给你鞠了一躬，你摆手说快上车。"


def ev_child_school(p: Person) -> str:
    p.shift(happiness=4, wealth=-3)
    return "孩子上小学，你在家长会上被老师点名，说你家孩子握笔姿势和你握拍一样歪。"


def ev_hometown(p: Person) -> str:
    p.shift(happiness=5, wealth=-2)
    return "回了一趟老家，老屋拆了，原地盖着别人家的二层楼，你在门口站了一会儿。"


def ev_reunion(p: Person) -> str:
    p.shift(happiness=6)
    return "老队友聚会，几个人凑不齐一桌，聊到最后都在说谁的膝盖先不行了。"


def ev_forty(p: Person) -> str:
    p.shift(happiness=3, health=-2)
    if p.has("coach"):
        return "四十岁生日，学生凑钱买了个蛋糕，蜡烛插成球拍的样子，你吹了三次才灭。"
    return "四十岁生日，你一个人在楼下的小饭馆点了碗面，加了个荷包蛋，算是过了。"


def ev_student_champ(p: Person) -> str:
    p.shift(happiness=9, wealth=4)
    return "你带的学生拿了全国少年组冠军，领奖时他回头找你，你在观众席上摆手让他看前面。"


def ev_mother_gone(p: Person) -> str:
    p.shift(happiness=-12, health=-2)
    return "母亲走了。整理遗物时你翻出一叠剪报，全是你比赛的豆腐块消息，按年份码得整整齐齐。"


def ev_checkup(p: Person) -> str:
    p.shift(health=-5, happiness=-3)
    return "体检加了心脏彩超，报告上写着建议复查，你把单子折起来放进抽屉最底下。"


def ev_old_friend(p: Person) -> str:
    p.shift(happiness=-5)
    return "老队友走了，葬礼上放的是你们当年比赛的录像，你在最后一排站着看完。"


def ev_travel(p: Person) -> str:
    p.shift(happiness=5, wealth=-4)
    return "和老伴报了个旅行团，第一次看见海。你在沙滩上走了很久，说比领奖台宽。"


def ev_memoir(p: Person) -> str:
    p.shift(happiness=3)
    return "有人来采访，想给你写口述史。你讲到一半停下来，说有些球其实不该那么打。"


def ev_photos(p: Person) -> str:
    p.shift(happiness=4)
    return "整理抽屉翻出体校的合影，背面用铅笔写着年份，你对着照片把人一个一个认过去。"


EVENTS: list[Event] = [
    Event("fever", 0, 6, lambda p: 3, ev_fever),
    Event("words", 0, 3, lambda p: 2, ev_words),
    Event("fall", 1, 4, lambda p: 3, ev_fall),
    Event("kindergarten", 3, 6, lambda p: 2.5, ev_kindergarten),
    Event("grandpa_ball", 4, 7, lambda p: 3 if not p.has("touched_racket") else 0, ev_grandpa_ball),
    Event(
        "coach_find",
        8,
        11,
        lambda p: 7 if p.pingpong >= 85 and not p.has("sports_school") else 0,
        ev_coach_find,
    ),
    Event("growth", 9, 14, lambda p: 2.5, ev_growth),
    Event("glasses", 9, 14, lambda p: 2.5, ev_glasses),
    Event("math", 9, 13, lambda p: 2 if p.intelligence >= 60 else 0.5, ev_math),
    Event("bully", 8, 12, lambda p: 2, ev_bully),
    Event(
        "first_match",
        10,
        13,
        lambda p: 4 if p.has("sports_school") else 0,
        ev_first_match,
    ),
    Event(
        "provincial",
        13,
        15,
        lambda p: 6 if p.has("sports_school") and p.physique >= 55 else 0,
        ev_provincial,
    ),
    Event(
        "basketball",
        13,
        16,
        lambda p: 3 if p.height >= 185 else 0,
        ev_basketball,
    ),
    Event(
        "injury",
        14,
        30,
        lambda p: 3 if p.has("provincial") else 1,
        ev_injury,
    ),
    Event("crush", 14, 18, lambda p: 3, ev_crush),
    Event("rebel", 14, 17, lambda p: 2, ev_rebel),
    Event(
        "national",
        16,
        19,
        lambda p: 5 if p.has("provincial") and not p.has("injury") else 0,
        ev_national,
    ),
    Event(
        "first_love",
        19,
        26,
        lambda p: 3 if not p.has("first_love") else 0,
        ev_first_love,
    ),
    Event(
        "sponsor",
        20,
        30,
        lambda p: 3 if p.has("national") else 0,
        ev_sponsor,
    ),
    Event(
        "university",
        18,
        20,
        lambda p: 3 if not p.has("national") and p.intelligence >= 60 else 0,
        ev_university,
    ),
    Event(
        "worlds",
        21,
        30,
        lambda p: 3.5 if p.has("national") and not p.has("world_champion") else 0,
        ev_worlds,
        repeatable=True,
        cooldown=4,
    ),
    Event(
        "olympics",
        22,
        30,
        lambda p: 3 if p.has("national") else 0,
        ev_olympics,
    ),
    Event("coach", 32, 45, lambda p: 5 if p.has("retired") and not p.has("coach") else 0, ev_coach),
    Event("marriage", 25, 38, lambda p: 4 if not p.has("married") else 0, ev_marriage),
    Event("child", 27, 40, lambda p: 4 if p.has("married") and not p.has("child") else 0, ev_child),
    Event("parents_ill", 30, 50, lambda p: 2.5 if not p.has("parents_ill") else 0, ev_parents_ill),
    Event(
        "knee",
        35,
        62,
        lambda p: 2.5 if p.has("provincial") and not p.has("knee") else 0,
        ev_knee,
    ),
    Event("invest", 30, 52, lambda p: 2.5, ev_invest),
    Event("child_school", 33, 42, lambda p: 4 if p.has("child") and not p.has("child_school") else 0, ev_child_school),
    Event("team_grow", 36, 50, lambda p: 4 if p.has("coach") and not p.has("team_grow") else 0, ev_team_grow),
    Event("forty", 40, 42, lambda p: 5, ev_forty),
    Event(
        "student_champ",
        40,
        54,
        lambda p: 4 if p.has("coach") and not p.has("student_champ") else 0,
        ev_student_champ,
    ),
    Event("mother_gone", 45, 62, lambda p: 3 if not p.has("mother_gone") else 0, ev_mother_gone),
    Event("checkup", 46, 56, lambda p: 3, ev_checkup),
    Event("retire_life", 56, 62, lambda p: 5 if not p.has("retired_life") else 0, ev_retire_life),
    Event("grandchild", 58, 76, lambda p: 3 if p.has("child") and not p.has("grandchild") else 0, ev_grandchild),
    Event("chronic", 55, 68, lambda p: 3.2, ev_chronic),
    Event("eye", 62, 78, lambda p: 3, ev_eye),
    Event("hearing", 70, 86, lambda p: 3, ev_hearing),
    Event("old_friend", 60, 86, lambda p: 2, ev_old_friend),
    Event("reunion", 60, 84, lambda p: 2.5, ev_reunion),
    Event("travel", 58, 76, lambda p: 2.5, ev_travel),
    Event("hometown", 60, 82, lambda p: 2.2, ev_hometown),
    Event("memoir", 65, 86, lambda p: 2.5 if p.has("national") else 0, ev_memoir),
    Event("photos", 62, 86, lambda p: 2, ev_photos),
]


def birth(p: Person, rng: random.Random) -> None:
    p.say(f"{p.name}出生了，是个{p.gender}。")
    p.say("护士把你抱出来时说，这孩子手脚比同病房的婴儿长出一截。")
    p.say("满月那天，爷爷把一只旧球拍塞进你手里，说先拿着玩。")


def milestone(p: Person, rng: random.Random) -> None:
    if p.age == 7 and not p.has("sports_school"):
        p.say("上小学了，班主任把你安排在第一排，因为再往后坐，后排同学就看不见黑板。")
        p.mark("primary")
    if p.age == 9 and p.has("sports_school"):
        p.say("正式进体校，每天五点半起床，跑完三千米再摸球拍。")
    if p.age == 13:
        p.say("上中学了，你的课本里总夹着一块擦拍子的海绵。")
    if p.age == 18 and not p.has("national") and not p.has("university"):
        p.mark("working")
        p.say("没打进省队，书也没再念下去，你去了一家体育用品店做店员，先养活自己。")
    if p.age == 31 and p.has("national") and not p.has("retired"):
        p.mark("retired")
        p.say("膝盖的核磁结果出来，队医说不能再练了。你在训练馆坐到熄灯。")


def tick(p: Person, rng: random.Random) -> None:
    milestone(p, rng)
    if p.age >= 56 and rng.random() < 0.3:
        p.shift(health=-1)
    pool = []
    for event in EVENTS:
        if not (event.start <= p.age <= event.end):
            continue
        if not event.repeatable and p.has(f"ev:{event.key}"):
            continue
        if event.repeatable:
            last = p.last_event.get(event.key)
            if last is not None and p.age - last < event.cooldown:
                continue
        weight = event.weight(p)
        if weight > 0:
            pool.append((event, weight))
    if not pool:
        return
    total = sum(weight for _, weight in pool)
    if rng.random() > 0.62:
        return
    roll = rng.uniform(0, total)
    for event, weight in pool:
        roll -= weight
        if roll <= 0:
            text = event.run(p)
            if text:
                p.say(text)
            p.mark(f"ev:{event.key}")
            p.last_event[event.key] = p.age
            p.tries[event.key] = p.tries.get(event.key, 0) + 1
            return


def lifespan_of(p: Person, rng: random.Random) -> int:
    base = rng.randint(72, 94)
    bonus = (p.health - 70) // 6 + (p.physique - 55) // 8
    return base + bonus


def die(p: Person, cause: str) -> None:
    p.alive = False
    p.cause = cause
    p.say(f"{cause}。这一年你 {p.age} 岁。")


def simulate(seed: int, name: str) -> Person:
    rng = random.Random(seed)
    p = Person(name=name, gender=rng.choice(["男孩", "女孩"]))
    p.health = 72 + rng.randint(-6, 8)
    p.intelligence = 52 + rng.randint(-10, 14)
    p.physique = 58 + rng.randint(-6, 10)
    p.charm = 50 + rng.randint(-8, 12)
    p.happiness = 62 + rng.randint(-8, 10)
    p.wealth = rng.choice([18, 30, 38, 52])
    p.height = rng.randint(183, 196)
    p.pingpong = rng.randint(92, 99)
    p.math = rng.randint(35, 65)
    p.music = rng.randint(20, 60)

    birth(p, rng)
    limit = lifespan_of(p, rng)
    while p.alive and p.age <= limit:
        tick(p, rng)
        p.age += 1
        if p.health <= 0:
            die(p, "旧伤和血压一起把你留在了医院，出院单始终没能签成")
            break
    if p.alive:
        die(p, "你走了，在一个再普通不过的清晨，家人说你睡着时脸上没什么痛苦")

    if p.age > 90:
        p.say("你的名字最后一次出现在市体校的老照片墙上。")
    return p


def stage_of(age: int) -> str:
    if age <= 6:
        return "幼年"
    if age <= 12:
        return "童年"
    if age <= 18:
        return "少年"
    if age <= 22:
        return "青年"
    if age <= 35:
        return "而立"
    if age <= 55:
        return "中年"
    return "晚年"


def render(p: Person) -> str:
    lines: list[str] = []
    lines.append(f"════════ {p.name} 的一生 ════════")
    lines.append("")
    current = ""
    for age, text in p.story:
        stage = stage_of(age)
        if stage != current:
            current = stage
            lines.append("")
            lines.append(f"—— {stage} ——")
        lines.append(f"{age:>3} 岁  {text}")
    lines.append("")
    lines.append("════════ 一生概览 ════════")
    lines.append(f"姓名：{p.name}    性别：{p.gender}    享年：{p.age} 岁")
    lines.append(f"身高：{p.height} cm    乒乓球天赋：{p.pingpong}")
    achievements = []
    if p.has("olympic_gold"):
        achievements.append("奥运冠军")
    if p.has("world_champion"):
        achievements.append("世乒赛冠军")
    if p.has("national"):
        achievements.append("国家队")
    if p.has("coach"):
        achievements.append("省队教练")
    if p.has("university"):
        achievements.append("大学生")
    if p.has("married"):
        achievements.append("已婚")
    if p.has("child"):
        achievements.append("有子女")
    lines.append("成就：" + ("、".join(achievements) if achievements else "平凡度日"))
    lines.append(
        f"最终：健康 {p.health}  智力 {p.intelligence}  体质 {p.physique}  "
        f"魅力 {p.charm}  快乐 {p.happiness}  财富 {p.wealth}"
    )
    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser(description="人生模拟器")
    parser.add_argument("--seed", type=int, default=None, help="随机种子，固定后可复现同一段人生")
    parser.add_argument("--name", default="陈默", help="角色姓名")
    args = parser.parse_args()
    seed = args.seed if args.seed is not None else random.randrange(10**6)
    person = simulate(seed, args.name)
    print(render(person))
    print(f"\nseed = {seed}")


if __name__ == "__main__":
    main()
