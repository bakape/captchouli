#pragma once
#include <stddef.h>

typedef struct {
    void* data;
    size_t size;
} Buffer;

char* thumbnail(void* classifier, const char* path, Buffer* thumb);

void* load_classifier(const char* path);
void unload_classifier(void* c);
