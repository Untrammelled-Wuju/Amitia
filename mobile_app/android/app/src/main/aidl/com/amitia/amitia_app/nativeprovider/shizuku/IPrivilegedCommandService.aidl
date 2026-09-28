package com.amitia.amitia_app.nativeprovider.shizuku;

interface IPrivilegedCommandService {
    String executeCommand(String requestJson);
    String startProcess(String requestJson);
    String writeProcess(String requestJson);
    String readProcess(String requestJson);
    String waitProcess(String requestJson);
    String killProcess(String requestJson);
    void destroy();
}
