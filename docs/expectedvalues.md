# Expected Values

An expected value is a statement which is utilised by verification rules.  

An expected value links one element with one intent as denoted in the fields

## Compulsory Fields

The compulsory fields are described below. The field *itemid* is generated by Jane and is a UUID4; this field should not be specified when adding a new element.

| Field | Type | Description |
| --- | --- | --- |
| itemid | UUID4 | Internally generated identifier |
| name | String | A short name for the element |
| description | String | A longer, more descripive description |
| elementid | UUID4 | A layer-7 operation name that the attestable element should understand |
| intentid | UUID4 | A dictionary object containing potential parameters for the function |
| evs | Dictionary | A dictionary object containing potential parameters for the function |

## EVS Section

This is a dictionary of values utilised by various rules - the specific names and semantics are defined by those rules. Common fields are:

| Field Name | Description |
| --- | --- |
| attestedValue | Value for the TPM 2.0 Quote attested value field, used with pcrQuotes |
| firmwareVersion | TPM 2.0 firmware version number, used with pcrQuotes |

Refer to the code linked in the next section and rule definitions for more field details

## Go Definition

The definition of the expected value structure can be found at https://github.com/iolivergithub/jane/blob/main/janeserver/structures/expectedValues.go

## Example

```json
{ "itemid" : "d4b76ce1-2b56-4346-aa11-546ea412d1e5", 
  "name" : "Work laptop firmware", 
  "description" : "Firmware values, specifically PCR0,2,4,7 for work laptop", 
  "elementid" : "c7219a2b-7d02-4c1f-bad7-50c4d3b2cd2a", 
   "intentid" : "6770b5b9-d0ea-4f8b-817f-cab1bf8169c0"  ,
  "evs" : { 
      "attestedValue" : "lR+zAH+TVf7g+QTNDVs9J0hD+VcbYNthM1RUQ2nfrsY=", 
      "firmwareVersion" : "19984793217276456" }
   }

```