#pragma once
#include <stddef.h>

typedef struct {
    void* data;
    size_t size;
} Buffer;

char* cpli_thumbnail(void* classifier, const char* path, Buffer* thumb);

void* cpli_load_classifier(const char* path);
void cpli_unload_classifier(void* c);
