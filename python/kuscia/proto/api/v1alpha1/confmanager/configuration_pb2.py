# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: kuscia/proto/api/v1alpha1/confmanager/configuration.proto
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import message as _message
from google.protobuf import reflection as _reflection
from google.protobuf import symbol_database as _symbol_database
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from kuscia.proto.api.v1alpha1 import common_pb2 as kuscia_dot_proto_dot_api_dot_v1alpha1_dot_common__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n9kuscia/proto/api/v1alpha1/confmanager/configuration.proto\x12%kuscia.proto.api.v1alpha1.confmanager\x1a&kuscia/proto/api/v1alpha1/common.proto\"s\n\x1a\x43reateConfigurationRequest\x12\x38\n\x06header\x18\x01 \x01(\x0b\x32(.kuscia.proto.api.v1alpha1.RequestHeader\x12\n\n\x02id\x18\x02 \x01(\t\x12\x0f\n\x07\x63ontent\x18\x03 \x01(\t\"P\n\x1b\x43reateConfigurationResponse\x12\x31\n\x06status\x18\x01 \x01(\x0b\x32!.kuscia.proto.api.v1alpha1.Status\"<\n\x19QueryConfigurationRequest\x12\x0b\n\x03ids\x18\x01 \x03(\t\x12\x12\n\ngroup_name\x18\x02 \x01(\t\"\xb6\x02\n\x1aQueryConfigurationResponse\x12\x31\n\x06status\x18\x01 \x01(\x0b\x32!.kuscia.proto.api.v1alpha1.Status\x12m\n\x0e\x63onfigurations\x18\x02 \x03(\x0b\x32U.kuscia.proto.api.v1alpha1.confmanager.QueryConfigurationResponse.ConfigurationsEntry\x1av\n\x13\x43onfigurationsEntry\x12\x0b\n\x03key\x18\x01 \x01(\t\x12N\n\x05value\x18\x02 \x01(\x0b\x32?.kuscia.proto.api.v1alpha1.confmanager.QueryConfigurationResult:\x02\x38\x01\"M\n\x18QueryConfigurationResult\x12\x0f\n\x07success\x18\x01 \x01(\x08\x12\x0f\n\x07\x65rr_msg\x18\x02 \x01(\t\x12\x0f\n\x07\x63ontent\x18\x03 \x01(\t2\xd1\x02\n\x14\x43onfigurationService\x12\x9c\x01\n\x13\x43reateConfiguration\x12\x41.kuscia.proto.api.v1alpha1.confmanager.CreateConfigurationRequest\x1a\x42.kuscia.proto.api.v1alpha1.confmanager.CreateConfigurationResponse\x12\x99\x01\n\x12QueryConfiguration\x12@.kuscia.proto.api.v1alpha1.confmanager.QueryConfigurationRequest\x1a\x41.kuscia.proto.api.v1alpha1.confmanager.QueryConfigurationResponseBb\n#org.secretflow.v1alpha1.confmanagerZ;github.com/secretflow/kuscia/proto/api/v1alpha1/confmanagerb\x06proto3')



_CREATECONFIGURATIONREQUEST = DESCRIPTOR.message_types_by_name['CreateConfigurationRequest']
_CREATECONFIGURATIONRESPONSE = DESCRIPTOR.message_types_by_name['CreateConfigurationResponse']
_QUERYCONFIGURATIONREQUEST = DESCRIPTOR.message_types_by_name['QueryConfigurationRequest']
_QUERYCONFIGURATIONRESPONSE = DESCRIPTOR.message_types_by_name['QueryConfigurationResponse']
_QUERYCONFIGURATIONRESPONSE_CONFIGURATIONSENTRY = _QUERYCONFIGURATIONRESPONSE.nested_types_by_name['ConfigurationsEntry']
_QUERYCONFIGURATIONRESULT = DESCRIPTOR.message_types_by_name['QueryConfigurationResult']
CreateConfigurationRequest = _reflection.GeneratedProtocolMessageType('CreateConfigurationRequest', (_message.Message,), {
  'DESCRIPTOR' : _CREATECONFIGURATIONREQUEST,
  '__module__' : 'kuscia.proto.api.v1alpha1.confmanager.configuration_pb2'
  # @@protoc_insertion_point(class_scope:kuscia.proto.api.v1alpha1.confmanager.CreateConfigurationRequest)
  })
_sym_db.RegisterMessage(CreateConfigurationRequest)

CreateConfigurationResponse = _reflection.GeneratedProtocolMessageType('CreateConfigurationResponse', (_message.Message,), {
  'DESCRIPTOR' : _CREATECONFIGURATIONRESPONSE,
  '__module__' : 'kuscia.proto.api.v1alpha1.confmanager.configuration_pb2'
  # @@protoc_insertion_point(class_scope:kuscia.proto.api.v1alpha1.confmanager.CreateConfigurationResponse)
  })
_sym_db.RegisterMessage(CreateConfigurationResponse)

QueryConfigurationRequest = _reflection.GeneratedProtocolMessageType('QueryConfigurationRequest', (_message.Message,), {
  'DESCRIPTOR' : _QUERYCONFIGURATIONREQUEST,
  '__module__' : 'kuscia.proto.api.v1alpha1.confmanager.configuration_pb2'
  # @@protoc_insertion_point(class_scope:kuscia.proto.api.v1alpha1.confmanager.QueryConfigurationRequest)
  })
_sym_db.RegisterMessage(QueryConfigurationRequest)

QueryConfigurationResponse = _reflection.GeneratedProtocolMessageType('QueryConfigurationResponse', (_message.Message,), {

  'ConfigurationsEntry' : _reflection.GeneratedProtocolMessageType('ConfigurationsEntry', (_message.Message,), {
    'DESCRIPTOR' : _QUERYCONFIGURATIONRESPONSE_CONFIGURATIONSENTRY,
    '__module__' : 'kuscia.proto.api.v1alpha1.confmanager.configuration_pb2'
    # @@protoc_insertion_point(class_scope:kuscia.proto.api.v1alpha1.confmanager.QueryConfigurationResponse.ConfigurationsEntry)
    })
  ,
  'DESCRIPTOR' : _QUERYCONFIGURATIONRESPONSE,
  '__module__' : 'kuscia.proto.api.v1alpha1.confmanager.configuration_pb2'
  # @@protoc_insertion_point(class_scope:kuscia.proto.api.v1alpha1.confmanager.QueryConfigurationResponse)
  })
_sym_db.RegisterMessage(QueryConfigurationResponse)
_sym_db.RegisterMessage(QueryConfigurationResponse.ConfigurationsEntry)

QueryConfigurationResult = _reflection.GeneratedProtocolMessageType('QueryConfigurationResult', (_message.Message,), {
  'DESCRIPTOR' : _QUERYCONFIGURATIONRESULT,
  '__module__' : 'kuscia.proto.api.v1alpha1.confmanager.configuration_pb2'
  # @@protoc_insertion_point(class_scope:kuscia.proto.api.v1alpha1.confmanager.QueryConfigurationResult)
  })
_sym_db.RegisterMessage(QueryConfigurationResult)

_CONFIGURATIONSERVICE = DESCRIPTOR.services_by_name['ConfigurationService']
if _descriptor._USE_C_DESCRIPTORS == False:

  DESCRIPTOR._options = None
  DESCRIPTOR._serialized_options = b'\n#org.secretflow.v1alpha1.confmanagerZ;github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager'
  _QUERYCONFIGURATIONRESPONSE_CONFIGURATIONSENTRY._options = None
  _QUERYCONFIGURATIONRESPONSE_CONFIGURATIONSENTRY._serialized_options = b'8\001'
  _CREATECONFIGURATIONREQUEST._serialized_start=140
  _CREATECONFIGURATIONREQUEST._serialized_end=255
  _CREATECONFIGURATIONRESPONSE._serialized_start=257
  _CREATECONFIGURATIONRESPONSE._serialized_end=337
  _QUERYCONFIGURATIONREQUEST._serialized_start=339
  _QUERYCONFIGURATIONREQUEST._serialized_end=399
  _QUERYCONFIGURATIONRESPONSE._serialized_start=402
  _QUERYCONFIGURATIONRESPONSE._serialized_end=712
  _QUERYCONFIGURATIONRESPONSE_CONFIGURATIONSENTRY._serialized_start=594
  _QUERYCONFIGURATIONRESPONSE_CONFIGURATIONSENTRY._serialized_end=712
  _QUERYCONFIGURATIONRESULT._serialized_start=714
  _QUERYCONFIGURATIONRESULT._serialized_end=791
  _CONFIGURATIONSERVICE._serialized_start=794
  _CONFIGURATIONSERVICE._serialized_end=1131
# @@protoc_insertion_point(module_scope)
