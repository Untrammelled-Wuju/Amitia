package com.amitia.amitia_app.runtime.proot

interface ProotProcessLauncher { fun launch(command: ProotCommand, observer: ProotObserver): ProotSession }