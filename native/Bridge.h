#ifndef SKGO_APPLICATION_BRIDGE_H
#define SKGO_APPLICATION_BRIDGE_H
#include <stdlib.h>
char *SKGoHostStart(void);
char *SKGoHostServerURL(void);
int SKGoHostRecover(void);
void SKGoHostStop(void);
#endif
