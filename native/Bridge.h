#ifndef SKGO_APPLICATION_BRIDGE_H
#define SKGO_APPLICATION_BRIDGE_H
#include <stdlib.h>
char *SKGoHostStart(void);
char *SKGoHostServerURL(void);
int SKGoHostRecover(void);
void SKGoHostStop(void);
void SKGoHostDiagnostic(const char *component, const char *message);
char *SKGoHostLogsDirectory(void);
char *SKGoHostExportDiagnostics(const char *destination);
#endif
