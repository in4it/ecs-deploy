import { Component, OnInit } from '@angular/core';
import {HttpClient, HttpHeaders } from '@angular/common/http';

import { AuthService } from '../services/auth.service';

import { Injectable }             from '@angular/core';
import { Observable }             from 'rxjs';
import { Router, RouterStateSnapshot, ActivatedRouteSnapshot } from '@angular/router';


import { ServiceList, ServiceListService }  from './service-list.service';


@Injectable()
export class ServiceListResolver  {

  constructor(private ds: ServiceListService) {}

  resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<ServiceList> {
    return this.ds.getServiceList().asObservable()
  }
  
}
