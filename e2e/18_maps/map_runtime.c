#include "map_runtime.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MAP_INITIAL_CAPACITY 16

typedef struct MapEntry {
  char *key;
  int64_t value;
  struct MapEntry *next;
} MapEntry;

// Simple hash function for strings
static unsigned int hash_string(const char *str, int capacity) {
  unsigned int hash = 5381;
  int c;
  while ((c = *str++)) {
    hash = ((hash << 5) + hash) + c;
  }
  return hash % capacity;
}

// Create a new map
Map *map_create() {
  Map *m = (Map *)malloc(sizeof(Map));
  m->capacity = MAP_INITIAL_CAPACITY;
  m->size = 0;
  m->buckets = (MapEntry **)calloc(MAP_INITIAL_CAPACITY, sizeof(MapEntry *));
  return m;
}

// Insert or update a key-value pair
void map_insert(Map *map, const char *key, int64_t value) {
  unsigned int index = hash_string(key, map->capacity);

  // Check if key exists
  MapEntry *entry = map->buckets[index];
  while (entry != NULL) {
    if (strcmp(entry->key, key) == 0) {
      entry->value = value;
      return;
    }
    entry = entry->next;
  }

  // Create new entry
  MapEntry *new_entry = (MapEntry *)malloc(sizeof(MapEntry));
  new_entry->key = strdup(key);
  new_entry->value = value;
  new_entry->next = map->buckets[index];
  map->buckets[index] = new_entry;
  map->size++;
}

// Get a value by key - returns the value directly (0 if not found)
int64_t map_get(Map *map, const char *key) {
  unsigned int index = hash_string(key, map->capacity);

  MapEntry *entry = map->buckets[index];
  while (entry != NULL) {
    if (strcmp(entry->key, key) == 0) {
      return entry->value;
    }
    entry = entry->next;
  }

  return 0;
}

// Free the map
void map_free(Map *map) {
  for (int i = 0; i < map->capacity; i++) {
    MapEntry *entry = map->buckets[i];
    while (entry != NULL) {
      MapEntry *next = entry->next;
      free(entry->key);
      free(entry);
      entry = next;
    }
  }
  free(map->buckets);
  free(map);
}
