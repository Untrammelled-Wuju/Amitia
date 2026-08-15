package com.amitia.amitia_app.nativeprovider.shizuku;

interface IPrivilegedCommandService {
    String executeCommand(String requestJson);
    void destroyService();
}
