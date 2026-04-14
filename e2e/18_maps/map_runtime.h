#ifndef MAP_RUNTIME_H
#define MAP_RUNTIME_H

#include <stdint.h>

typedef struct MapEntry MapEntry; // Forward declaration for MapEntry

typedef struct Map {
  MapEntry **buckets;
  int capacity;
  int size;
} Map;

Map *map_create();
void map_insert(Map *map, const char *key, int64_t value);
int64_t map_get(Map *map, const char *key);
void map_free(Map *map);

#endif
